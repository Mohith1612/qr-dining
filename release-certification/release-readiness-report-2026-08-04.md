# Release Readiness Report — 2026-08-04

**Release Candidate:** `feature/signoz-observability` @ `c4aeb4b`
**Pull Request:** [#1](https://github.com/Mohith1612/qr-dining/pull/1) → `main` (open, **not merged**)
**Scope of this report:** merge readiness only. Tagging, deployment, and the RC soak are
the next phase and were deliberately not started.

## Merge recommendation

**Conditional GO — merge is safe; one security item should be resolved first.**

The mechanical case for merging is strong and now evidence-backed rather than assumed:

- `main` is **0 commits ahead** of the RC, so the merge is a clean fast-forward with no
  conflicts.
- All **10 CI checks pass**, on the first run the repository has ever had.
- The pipeline now actually exercises the code this RC exists to ship — the cross-tenant
  authz regression tests run in CI for the first time (42 subtests, 0 skipped).
- The working tree is clean, no secrets or build artifacts are tracked, and all 39
  migrations have matched up/down pairs.

The condition is a dependency vulnerability finding described below. It is a two-line
change, but it is out of the scope you set for this session ("fix only issues discovered
by CI"), so I have reported it rather than applied it.

## Blockers

### B1 — Reachable `x/text` vulnerability in the startup path (recommend fixing before merge)

CI has no vulnerability scanning, so this was found by running `govulncheck` by hand.
Scanned against the **actual release toolchain** (go1.26.5, the version both CI and the
Docker builder resolve to):

| ID | Module | Found | Fixed | Reachability |
|---|---|---|---|---|
| GO-2026-5970 | `golang.org/x/text` | v0.37.0 | v0.39.0 | `db.RunMigrations` → `stdlib.OpenDBFromPool` → `norm.Form.*` |
| GO-2026-6061 | `google.golang.org/grpc` | v1.81.1 | v1.82.1 | `internal/observability` → OTLP gRPC exporter |

**GO-2026-5970 is the one that matters.** It is an infinite loop on invalid input, and
the call path runs at process startup on every boot, not behind a feature flag.

**GO-2026-6061 is latent.** `go mod why` shows grpc is reachable only through the OTLP
gRPC trace exporter, and `OTEL_ENABLED` defaults to `false` (`internal/config/config.go:203`),
so the exporter is never constructed in the shipped configuration. It becomes live the
moment observability Phase 3 turns OTEL on — which is planned, so this should not be
carried indefinitely.

Both are indirect dependencies; the fix is a `go get` of each to the fixed version plus a
re-run of the pipeline.

**A note on scan hygiene:** an initial scan on my local toolchain (go1.26.0) reported
21 vulnerabilities. Nineteen were standard-library issues fixed in go1.26.1 and do **not**
affect the release artifact, because `setup-go` with `"1.26"` and `golang:1.26-alpine`
both resolve to go1.26.5. Only the two above are real. Any future scanner added to CI
must run against the resolved toolchain or it will produce the same false picture.

## Non-blocking but important

### R1 — A green pipeline does not prove this release candidate

This is the single most important caveat in this report. CI is green and CI is now
meaningfully better than it was, but:

- **387 Playwright tests are discovered and none are executed.** `frontend-ci.yml` runs
  `playwright test --list` only. The unexecuted specs include `tenancy/T-04..06` and
  `webhook/W-01..07` — the isolation and payment-settlement guarantees this RC claims to
  fix. The behavioural evidence for those claims is currently manual testing, not CI.
- **OpenAPI lint proves the spec is valid, not that it matches the server.** The 157
  operations ↔ `server.go` 1:1 correspondence remains a manual certification.
- **No automated security scanning**, which is how B1 survived to this point.

### R2 — `000039` is a forward-only data migration

`000039_normalize_promo_redemption_phones` rewrites `promo_redemptions.phone_e164` in
place and nulls rows holding no digits. Its `down` is intentionally a no-op: the original
strings are unrecoverable, and restoring them would re-open the per-phone promo cap
bypass.

The CI Migration Check runs up → down -all → up against an empty database, so it proves
schema reversibility and cannot detect this. **Rolling back across `000039` requires a
restore from backup, not `migrate down`.** Take a verified backup before applying it.

### R3 — Behavioural change in authorization

Scope denials now return 403 even where `AUTHZ_CENTRAL_POLICY_ENFORCE` is off. This is
the intended fix, but it is a behavioural change rather than a no-op: anything that was
relying on cross-branch or cross-organization access — including internal tooling or
scripts — will now break. The shadow-mode signal that would ordinarily surface this in
advance no longer applies to scope violations, by design.

### R4 — No branch protection on `main`

Nothing enforces that these 10 checks pass before a merge. The pipeline is advisory. This
should be configured as part of the merge phase, together with a decision on whether the
path-filtered `frontend-ci.yml` jobs are required checks — if they are, backend-only PRs
will block on skipped runs.

### R5 — The publish path has never executed

`push: true` only evaluates on a push to `main` or a `v*` tag. GHCR login, image push, and
tag metadata have therefore never run. The first tag will exercise that path for the first
time — expect to debug it there rather than assume it works, and do not schedule it tightly
against the soak start.

## Operational concerns

- **Frontend does not ship on merge.** Cloudflare deployment is manual per
  `DEPLOYMENT.md` §7; there is no deploy workflow. Merging this PR ships nothing by itself.
- **Image is `linux/arm64` only**, matching the Oracle Ampere target. Correct, but it
  cannot be run directly on an amd64 host without emulation — worth knowing before someone
  tries to reproduce a production issue locally.
- **Observability ships disabled.** `OTEL_ENABLED=false`; Phase 0/3 remain blocked on VM
  provisioning, so the RC soak will not have SigNoz traces unless that is resolved first.
- **Soak the RC build itself.** The prior soak ran a pre-redesign binary. Soaking anything
  other than the artifact built from this commit would not certify this release.

## Confidence level

**High** that the merge is mechanically safe: clean fast-forward, 10/10 green, no secrets,
no artifacts, migrations paired, working tree clean.

**High** that the backend is exercised. Unit tests pass under `-race`, and the integration
suite — which had never run in CI — now runs and demonstrably executes rather than skips.

**Moderate** that the *system* is proven. The end-to-end behavioural evidence is manual,
because Playwright does not execute in CI. For a first restaurant deployment, that is the
gap I would most want closed before or during the soak.

**The pipeline is now trustworthy in what it asserts.** Its remaining weakness is scope,
not correctness: it tells the truth about the backend, and stays silent about the browser.

## Recommended next steps

In order:

1. **Resolve B1** — bump `golang.org/x/text` to ≥ v0.39.0 and `google.golang.org/grpc` to
   ≥ v1.82.1, push, and confirm the pipeline stays green. Small and low-risk.
2. **Add `govulncheck` to CI** so B1-class findings cannot recur silently. This is the
   highest-value workflow still missing, and it is cheap.
3. **Configure branch protection on `main`** with the 7 CI jobs required, and decide the
   `frontend-ci.yml` required-checks question at the same time.
4. **Merge PR #1** once 1–3 are settled.
5. **Tag the release** and watch the publish path closely — it has never executed.
6. **Take a verified backup**, then apply migrations through `000039`.
7. **Begin the RC soak on the artifact built from this commit**, not a prior binary.
8. **Later, not blocking:** run Playwright against a real stack in CI, add SBOM
   generation, and SHA-pin actions.

Items 5–7 are the next release phase and were deliberately not started here. No merge, no
tag, no deploy, and no soak was performed in this session.
