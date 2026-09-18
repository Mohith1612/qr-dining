# Bringing the qr-dining vhost under provenance

`proxy_nginx` is **not** in any compose file in this repository, and it cannot
be. It is the shared reverse proxy for the whole VM — on 2026-09-18 it fronted
five projects:

| vhost in `/opt/proxy/nginx/conf.d/` | owner |
| --- | --- |
| `qr-dining-beta.conf` | this repository |
| `grafana.conf` | unified-monitoring-agent |
| `job-queue.conf` | job-queue-system |
| `sketchiple.conf` | sketchiple |
| `000-catchall.conf` | `/opt/proxy` itself |

Its lifecycle is owned by `/opt/proxy/docker-compose.yml`, which also runs
certbot and holds the `letsencrypt` and `certbot_webroot` volumes. This repo
does not get to own that container, and claiming otherwise in a compose file
here would be a lie that breaks the first time someone runs `docker compose
down` from `/opt/qr-dining`.

**What this repo can own: the one vhost file that is ours.** Nothing else.

## What was found

The live `/opt/proxy/nginx/conf.d/qr-dining-beta.conf` was a **hand copy** of
`deploy/vm/qr-dining-beta.conf.example`. It happened to be byte-identical to the
repo's example on 2026-09-18 — but nothing on the box could establish that. No
command answered "which commit is the running routing config?", which is the
same defect that left Prometheus evaluating 27 of 32 alert rules for three days
(docs/OPERATIONS.md, "Keeping the VM in step with the repo").

## The change

Two edits to `/opt/proxy/docker-compose.yml`, which is **outside this
repository** — that is the boundary, and it is why this file exists instead of a
compose file you could just apply:

```yaml
services:
  nginx:
    volumes:
      - ./nginx/nginx.conf:/etc/nginx/nginx.conf:ro
      - ./nginx/conf.d:/etc/nginx/conf.d:ro
      - letsencrypt:/etc/letsencrypt
      - certbot_webroot:/var/www/certbot
      # ADDED: qr-dining's vhost, mounted from the git checkout that is already
      # production state for this project's observability config.
      #
      # A DIRECTORY, not the file. A single-file bind mount pins the inode it
      # was created with, and `git checkout` REPLACES files rather than
      # rewriting them in place — so the container would go on serving a version
      # that no longer exists on disk. Reproduced on this VM; see
      # docs/OPERATIONS.md, "Why the nginx mount is a directory".
      - /opt/qr-dining/repo/deploy/vm/nginx:/etc/nginx/qr-dining.d:ro
```

and then, once:

```bash
# The stub replaces the hand copy. It is three lines and never changes again.
cp /opt/qr-dining/repo/deploy/vm/proxy-nginx/qr-dining-beta.conf.stub \
   /opt/proxy/nginx/conf.d/qr-dining-beta.conf

# Recreating proxy_nginx is the only way to add a mount, and it is a few seconds
# of downtime for FOUR OTHER PROJECTS. Do it deliberately, not during an
# incident, and validate before and after.
docker compose -f /opt/proxy/docker-compose.yml up -d nginx
docker exec proxy_nginx nginx -t
```

## What this buys, and what it costs

Buys: `git log -1 -- deploy/vm/nginx/` answers "what routing is running", and
`git pull` is the deploy. `deploy/vm/deploy.sh` fast-forwards that checkout on
every deploy, so routing changes ship with the code that needs them.

Costs, stated plainly: **a bad commit to `deploy/vm/nginx/` is now an outage for
grafana, job-queue and sketchiple as well as for qr-dining**, delivered by a
`git pull` nobody was watching. That is why `deploy/vm/deploy.sh` runs
`nginx -t` after every fast-forward, reverts the checkout if it fails, and
refuses to reload. Never reload this proxy by hand without `nginx -t` first.

## What is still outside provenance

- The `proxy_nginx` container, its image pin (`nginx:stable-alpine` — a floating
  tag), its ports, and `nginx.conf` itself.
- `000-catchall.conf`, and the other three projects' vhosts.
- certbot, the wildcard certificate and its DNS-01 renewal.

None of that is describable from this repository. A `git log` here does not
explain a change to any of it.
