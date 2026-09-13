# Testing

Last verified against the repository: 2026-09-13.

## Backend suites

Files without a build constraint form the unit/default suite:

```bash
cd backend
go test -count=1 -race ./...
```

CI verifies dependencies, builds all packages, and runs that exact race-enabled
test command (`.github/workflows/ci.yml:63-70`).

Database/Redis integration files begin with `//go:build integration`, for example
the credential-leak regression
(`backend/internal/handlers/session_credential_leak_integration_test.go:1-3`). Run
the tagged suite against disposable PostgreSQL 17 and Redis 7:

```bash
cd backend
TEST_DATABASE_URL='postgres://...' TEST_REDIS_URL='redis://...' \
  go test -count=1 -p 1 -race -tags integration ./...
```

`-p 1` is required, not a speed preference. All package test binaries share the
same database and Redis, and fixture cleanup issues `TRUNCATE ... CASCADE`; package
parallelism makes them truncate each other's state and can deadlock
(`.github/workflows/ci.yml:197-205`,
`backend/internal/testutil/db.go:39-49`). The CI integration job provides one
PostgreSQL and one Redis service and uses `-p 1`
(`.github/workflows/ci.yml:159-205`).

The migration job is narrower than an upgrade rehearsal: it runs up, down-all,
and up against a fresh empty database (`.github/workflows/ci.yml:125-157`). It
cannot reveal data-dependent replay failures such as migration 22 meeting a
populated immutable audit log; that tested recovery is in
[RECOVERY.md](RECOVERY.md).

The backup/restore regression harness is separate:

```bash
cd backend
scripts/tests/backup-restore-test.sh
```

It uses isolated source and target PostgreSQL containers and tests the four
Track F fixes (`backend/scripts/tests/backup-restore-test.sh:1-18,65-87`). It exits
zero with `SKIP` when Docker or Go is missing, so inspect its summary rather than
using the exit code alone (`backend/scripts/tests/backup-restore-test.sh:51-52,310-311`).

## Frontend and contract checks

The frontend CI job runs `npm ci`, lint, and production build; marketing runs
`npm ci` and its build (`.github/workflows/frontend-ci.yml:18-57`). The backend CI
also checks sqlc generation drift and lints `openapi.yaml`
(`.github/workflows/ci.yml:99-123,207-223`). OpenAPI lint proves syntax/ruleset
compliance, not router/spec parity; the workflow says so explicitly
(`.github/workflows/ci.yml:220-223`).

## Playwright topology

The e2e package runs `playwright test` and offers desktop, mobile, and headed
variants (`e2e/package.json:5-11`). Playwright is fully parallel with two workers,
retains failure screenshots/video/traces, and collects each declaration for
desktop, mobile, and tablet (`e2e/playwright.config.ts:6-22,25-47`). Start the
isolated two-backend/two-frontend stack before executing it; the stack endpoints
are fixed at 8090/8095 and 3000/3001
(`scripts/manual-testing-up.sh:1-10,62-80,105-123`).

CI currently does **not** execute the e2e cases. It installs dependencies and runs
only `playwright test --list` (`.github/workflows/frontend-ci.yml:59-76`). A green
CI workflow is therefore not an e2e pass.

## Vacuity census and quarantine

The audit found tests that could remain green when the behavior in their titles
was broken. The accepted signatures are: swallowed failure, conditional/no-
assertion paths, success/failure widening, nonexistent response fields, claims
without an attempt, unreachable fixture state, UI claims tested only by `fetch`,
and assertions that merely repeat seeded values
(`release-certification/prompts/step0-a-e2e-quarantine.md:49-81`).

Exactly 59 declarations are deliberately written as `test.fixme`, each preceded
by a greppable `VACUOUS(sig-N)` explanation. The audited breakdown is 52
`QUARANTINE` plus 7 `UNREACHABLE`
(`audit/e2e-vacuity-census.md:12-18,29-41,61-73`). `test.fixme` here means a known
broken test that must be repaired; the task explicitly rejected `test.skip`
because “not applicable” would hide the debt
(`release-certification/prompts/step0-a-e2e-quarantine.md:28-47,99-105`). Do not
turn one back into `test(...)` by merely widening its accepted outcome.

Reproduce the current declaration counts instead of copying an old total:

```bash
find e2e -type f -name '*.spec.ts' | wc -l
rg --glob '*.spec.ts' -c '^\s*test\s*\(' e2e | awk -F: '{s+=$2} END{print s+0}'
rg --glob '*.spec.ts' -c '^\s*test\.fixme\s*\(' e2e | awk -F: '{s+=$2} END{print s+0}'
(cd e2e && npx --no-install playwright test --list) | tail -n 1
```

At the 2026-09-12 census checkpoint the results were 109 spec files, 74 raw
`test` declarations, 59 `test.fixme`, and 399 project cases
(`audit/e2e-vacuity-census.md:20-57`). The current tree adds the executing F-07
staff-cancel UI declaration (`e2e/frontend/F-07-staff-cancels-payment-through-ui.spec.ts:15-99`),
so the reproducible current result is 110 files, 75 raw declarations, 59 fixmes,
and 402 cases across three projects. Of the 75 raw declarations, five screenshot
sweeps are `ARTIFACT`: they execute to produce images but are not correctness
coverage (`audit/e2e-vacuity-census.md:3-18`). That leaves 70 currently classified
trusted executing declarations, multiplied to 210 project cases; this is an
arithmetic update to the census, not a fresh adversarial review of F-07.

Six quarantined settlement declarations also sit under `test.describe.skip`
because those settlement scenarios are outside the current manual-confirmation
flow (`audit/e2e-vacuity-census.md:90-107`). Seven webhook signature/timestamp
checks are enabled because the public webhook route and its signature boundary
exist regardless of settlement integration
(`audit/e2e-vacuity-census.md:90-113`,
`backend/internal/server/server.go:235-247`).

## Historical identifier trap

The removed `docs-before-rebuild:docs/history/operational-correctness-audit.md` used a colliding
F-number scheme; for example, its F-27 was a theme-management item, not the newer
credential-leak finding. Always cite the ledger beside an F-number
(`audit/findings-ledger.md:9-13`). The removed audit remains retrievable only from
the `docs-before-rebuild` tag, so agents do not encounter it as current guidance.
