# Deploy drills

## Rehearsing the health gate and the automatic rollback

Builds an image that starts, stays up, and answers `/readyz` with **HTTP 200 and
a not-ready body**. Run on the VM:

```bash
cd /opt/qr-dining/repo/deploy/vm/drills

# Base it on whatever is currently deployed, so the ONLY difference under test
# is the readiness answer.
BASE="$(docker inspect qr_dining_app --format '{{.Config.Image}}')"
docker build --build-arg "BASE=$BASE" \
  -f Dockerfile.broken-readyz \
  -t qr-dining-drill:broken-readyz .

# Deploy it. --tag skips the CI gate and the config fast-forward (there is no
# commit), but the migration pre-flight and the health gate both run for real.
QRD_DEPLOY_IMAGE_REPO_OVERRIDE=qr-dining-drill \
  /opt/qr-dining/repo/deploy/vm/deploy.sh --tag broken-readyz
```

Expect: `qr_dining_app` is recreated, the gate rejects the body, `qr_dining_app_2`
is **never touched**, the previous image is restored on both instances, and a
red Telegram report arrives. The whole thing takes about three minutes, most of
it the `QRD_HEALTH_TIMEOUT` wait — set it lower to make the drill quicker:

```bash
QRD_HEALTH_TIMEOUT=45 QRD_DEPLOY_IMAGE_REPO_OVERRIDE=qr-dining-drill \
  /opt/qr-dining/repo/deploy/vm/deploy.sh --tag broken-readyz
```

Clean up afterwards:

```bash
docker rmi qr-dining-drill:broken-readyz
/opt/qr-dining/repo/deploy/vm/deploy.sh --clear-hold broken-readyz
```

The rollback places a hold on the tag it rolled away from, so that a real bad
build is not redeployed two minutes later. `--tag` deploys ignore holds, so the
leftover one is harmless — but `--holds` is cleaner without it.

**What the drill also shows, and you should watch for:** while the broken image
is up, it is still in the `split_clients` rotation and still serving. It answers
`404` for unknown paths exactly as the real app does — only the headers differ —
so neither nginx nor a client-side probe can tell the difference. The health
gate protects the deploy, not the users, for as long as it is deciding. See
docs/OPERATIONS.md, "The canary serves real traffic while it is being judged".

## Rehearsing the migration gate

No artefact needed — deploy any image whose highest migration exceeds
`schema_migrations.version`. If the database is current, the cheapest rehearsal
is to ask the pre-flight directly, which changes nothing:

```bash
docker run --rm --network qr-dining_qr-dining_internal \
  --env-file /opt/qr-dining/.env \
  --entrypoint /app/migrate ghcr.io/mohith1612/qr-dining:<tag> pending; echo "exit=$?"
```

`exit=10` is the gate firing.

## Why these live in the repo

Both drills were run against production on 2026-09-18 and both are expected to
be run again — after any change to `deploy/vm/deploy.sh`, and before any pilot
service that depends on a deploy having happened. A drill that exists only in
somebody's shell history is a drill that stops being run.
