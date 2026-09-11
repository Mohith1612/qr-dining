# Release Readiness Summary — RC Certification (2026-07-18)

- **RC:** `feature/certification-fixes-ui-redesign` @ `439de14` (was `9dd869a` at certification
  start; 5 certification-fix commits added, all listed in `fixed-during-certification.md`)
- **Certified by:** automated certification pass (build/test/lint/e2e + multi-role exploratory
  inspection), 2026-07-18

## Recommendation: **READY FOR MANUAL TESTING**

The RC is in the best automated-verification state it has ever been: every build, lint, unit,
race, sqlc, integration, frontend, and OpenAPI gate is green; the Playwright suite is at its
known-good ceiling (99/127, remainder documented spec-side); and a full multi-role exploratory
sweep found **zero new product defects**, with both pilot regressions (F-1 reconnect, F-6 live
tracker) verified fixed in the real UI across two backend instances.

It is **not** yet "ready for RC soak" only because that phase is explicitly gated on your manual
certification — nothing found this phase would block starting the soak once you sign off.

## Current confidence

| Area | Confidence | Basis |
|---|---|---|
| Backend core (session/order/payment/realtime) | **High** | full integration suite green (0 skips), e2e core green, exploration clean |
| Guest UI (Serene) | **High** | full journey exercised incl. rejoin, reactivation, promo, payment, ended screen |
| Staff UI (Harmony: waiter/kitchen/manager/owner) | **High** | all roles exercised; role scoping correct; live boards |
| Platform console | **Medium-High** | super_admin + support exercised; billing/auditor logins + MFA enrollment left for manual pass |
| Cross-instance realtime | **High** | guest on app-1, staff on app-2 throughout exploration |
| CI pipeline | **Medium** | gates restored (gofmt/golangci v2) but not yet run on GitHub for this branch |
| E2E suite signal | **Medium** | 28 documented spec-side failures remain (deferred to R4 cleanup) |

## Remaining blockers to v1.0 (full list in release-checklist.md)

1. **Your manual certification** using `manual-testing-checklist.md` — the gate for everything else.
2. **SEV-1 merge + tag** (`main` --no-ff, `v1.0.0-rc.1`) after your sign-off.
3. **SEV-0 fresh soak of the tagged build** — the single biggest release risk: redesign +
   migrations 000036–000038 + cert fixes have never soaked.
4. **SEV-1 real alert receiver** (Alertmanager still fires into a placeholder sink).
5. Production host provisioning + `production-environment-checklist.md` run.

## Recommended fixes (before or shortly after v1.0)

- Integer-paise money math (before scale).
- e2e spec cleanup per `docs/history/e2e-failure-analysis.md` + add integration/e2e slices to CI.

## Known limitations (accepted for supervised pilot)

- R2, R3, and R7 remain unflipped; R4–R6 are configured for launch with signed
  guest credentials, staff-code/session enforcement, and ticketed WS. No real payment gateway
  (cash/manual + simulated webhooks only); `session_sequences` hot-spot beyond ~150 concurrent;
  kitchen KDS is 10 s polling by design.

## Risk assessment

- **Low risk** in everything this phase could verify: core dine-in loop, realtime, role isolation,
  payments (manual paths), theming, admin surfaces.
- **Medium risk** where only the soak can answer: long-run stability of the redesigned frontend +
  new migrations under sustained traffic (SEV-0), and first CI run of the restored lint gates.
- **Deliberately deferred risk:** F-8 and unflipped strict waves — irrelevant to the supervised
  pilot, disqualifying for public exposure until R6.

## What changed during certification (all conservative, all committed individually)

`372e3be` gofmt 19 files · `ff23778` golangci v2 migration + 10 lint fixes · `a91cb7e` eslint
ignore `.open-next` · `f493dc9` integration-test rot fix · `439de14` e2e webhook contract
realignment. No product-behavior changes; the only product-code edits are dead-code removal, an
ineffectual assignment, and the deprecated Prometheus collector constructors (metrics identical).
