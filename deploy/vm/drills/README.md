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

Clean up afterwards: `docker rmi qr-dining-drill:broken-readyz`.

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
