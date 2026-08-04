# Dependency Security Report — 2026-08-04

**Release Candidate:** `feature/signoz-observability` @ `9f97a54`
**Pull Request:** [#1](https://github.com/Mohith1612/qr-dining/pull/1) → `main` (open, not merged)

## Summary

Both Go vulnerabilities carried by the previous release report are fixed and the build is
now gated so they cannot recur silently. One frontend upgrade was taken on top. Backend
`govulncheck` against the release toolchain goes from **2 affecting → 0 affecting**.

| | Before | After |
|---|---|---|
| Go vulnerabilities affecting code | 2 | **0** |
| Next.js direct advisories | 8 | **0** |
| Vulnerability scanning in CI | none | `govulncheck` gate |

## Part 1 — Vulnerabilities investigated

### GO-2026-5970 — `golang.org/x/text` — UPGRADED v0.37.0 → v0.39.0

**What it is.** CVE-2026-56852. `norm.Iter` can enter an infinite loop on input
containing invalid UTF-8 bytes. Affected symbols include `Form.Properties`, `Form.Span`,
and `Form.Transform` — exactly the three govulncheck traced.

**Does it affect this application? Yes, the path is live.** `go mod why` resolves it
through `internal/db/sqlc` → `jackc/pgx/v5/pgconn` → `golang.org/x/text/secure/precis`.
That is SASLprep, the normalization pgx runs over the password during SCRAM
authentication — so it executes on database connection, on every boot, not behind a
feature flag.

**Correction to the previous report,** which attributed this to `db.RunMigrations` alone.
Migrations are where govulncheck happened to root the trace, but the real reach is the
connection path, which is broader.

**Practical exploitability: low.** The input to SASLprep is our own configured credential
from the environment, not attacker-controlled data. An attacker would need to already
control the database password to trigger it. Reachable, but not remotely reachable.

**Upgrade safety: safe.** `x/text` is an indirect dependency; v0.37.0 → v0.39.0 is a minor
bump within the `golang.org/x` compatibility promise, with no API surface used directly by
this module. No breaking changes. Full battery re-run below.

### GO-2026-6061 — `google.golang.org/grpc` — UPGRADED v1.81.1 → v1.82.1

**What it is.** GHSA-hrxh-6v49-42gf. Vulnerabilities in the xDS RBAC authorization engine
and the HTTP/2 transport.

**Does it affect this application? Latent, not live.** Reachable only through the OTLP
gRPC trace exporter. `SetupTracing` returns a no-op before ever calling
`otlptracegrpc.New` when `cfg.Enabled` is false, and `OTEL_ENABLED` defaults to `false`
(`internal/config/config.go:203`), so no gRPC client is constructed in the shipped
configuration.

Exposure is lower still even with tracing on: the advisory covers the **xDS RBAC engine**
and the **HTTP/2 transport server**. This application runs no gRPC server and no xDS — it
would be a gRPC *client* to a collector on the same host.

**It does not stay latent.** Observability Phase 3 turns tracing on by design, so this
would become a live dependency of the deployed system. Fixing it now is cheaper than
remembering to fix it then.

**A finding that was noise.** govulncheck also traced grpc through
`internal/redis/pubsub.go` (`PubSub.Subscribe` → `PubSub.Channel` →
`transport.NewHTTP2Client`). That is call-graph over-approximation, not a real path:
`go list -deps github.com/redis/go-redis/v9` returns **zero** grpc packages. go-redis does
not depend on grpc at all. The previous session flagged this trace as a possible live
path; it is not.

**Upgrade safety: safe.** Indirect dependency, patch-level bump within the same minor.

### `golang.org/x/sync` v0.20.0 → v0.21.0 — forced, not chosen

This third module moved and is worth explaining, because it looks like scope creep.

It is not optional. `x/text` v0.39.0 declares `golang.org/x/sync v0.21.0` in its own
`go.mod`, so minimal version selection raises it. Verified by experiment: pinning
`x/sync` back to v0.20.0 drags `x/text` back to **v0.37.0** and reopens GO-2026-5970.

It is also not a risk. `x/sync` is used in exactly one place — `errgroup` in
`internal/services/session.go:21` — and `errgroup`'s API has been stable for years.

Notably, grpc v1.82.1 itself still only requires `x/sync` v0.20.0, so the bump comes
entirely from `x/text`.

### Next.js 15.5.18 → 15.5.22 — upgraded after scoping the real exposure

Eight advisories, all fixed in 15.5.21+. The offered fix is 15.5.22, `isSemVerMajor:
false`.

**Six do not apply to this application,** verified against the source rather than assumed:

| Advisory | Severity | Applies? |
|---|---|---|
| GHSA-m99w-x7hq-7vfj — App Router DoS via Server Actions | high | No — no `'use server'` in the tree |
| GHSA-89xv-2m56-2m9x — SSRF in Server Actions on custom servers | high | No — same |
| GHSA-4c39-4ccg-62r3 — Unbounded Server Action payload (Edge) | moderate | No — same |
| GHSA-955p-x3mx-jcvp — Disclosure of internal Server Function endpoints | moderate | No — same |
| GHSA-p9j2-gv94-2wf4 — SSRF in rewrites via attacker-controlled hostname | high | No — no rewrites configured |
| GHSA-q8wf-6r8g-63ch — Image Optimization DoS via SVG | moderate | No — no `next/image` usage |
| **GHSA-68g3-v927-f742 — Cache confusion for requests with bodies** | **moderate** | **Possibly** |
| **GHSA-4633-3j49-mh5q — Cache confusion, invalid UTF-8 bodies** | **moderate** | **Possibly** |

So the honest headline was never "8 high severity vulnerabilities in production" — it was
two moderate cache-confusion issues. The upgrade was taken anyway because it is a patch
bump that costs nothing and removes the whole class from the report.

**Result.** `next` now carries **zero** direct advisories. It still appears in `npm audit`
only because its own `postcss` and `sharp` dependencies are flagged — see Part 3.
Lockfile churn was confined to `next` and `@next/*`.

## Part 2 — Verification

Every item below was run locally against the upgraded tree. The Docker and integration
checks used throwaway containers on ports 55432/56379; the soak stack was not touched.

| Check | Result |
|---|---|
| `govulncheck` (go1.26.5) | **0 affecting**, was 2 |
| `go mod tidy` | clean, no further churn |
| `go mod verify` | all modules verified |
| `go build ./...` | pass |
| `gofmt -l .` / `go vet ./...` | clean |
| Unit tests `-race` | pass, 10/10 packages |
| Integration tests `-p 1 -race -tags integration` | pass, 12/12 packages incl. `worker`, `redis` |
| sqlc drift | no drift |
| arm64 Docker build | pass |
| Frontend `npm run build` | pass |
| Frontend `npm run lint` | pass, 0 errors |

**Dependency churn was minimal and audited:** exactly three Go modules changed and 12
`go.sum` lines; the npm lockfile changed only `next` and `@next/*`.

## Part 3 — Dependency review (recommendations only)

No mass upgrades performed. Nothing in this section was changed.

### Formally deprecated

**None.** `go list -m -f '{{.Deprecated}}' all` returns no Go-declared deprecations across
the entire module graph, and `npm ls` reports no deprecated frontend packages.

### Monitor — stale but current

These are already at their latest published version, so there is nothing to upgrade. The
concern is upstream velocity, not our lag.

| Package | Version | Last release | Why it matters |
|---|---|---|---|
| **`gorilla/websocket`** | v1.5.3 | **2024-06-14** | **Highest-priority monitor.** Over two years without a release, on the single most load-bearing dependency in a realtime, session-centric product. The project was archived in 2022 and revived under new maintainers, but momentum is low. `coder/websocket` is the actively maintained alternative. Not a migration to attempt before a soak — but if this stays quiet, it becomes a real succession question. |
| `joho/godotenv` | v1.5.1 | 2023-02-05 | Three and a half years quiet. Very low risk: its entire job is parsing `.env`, the format is frozen, and it is config-load only. Monitor, do not prioritise. |
| `google/uuid` | v1.6.0 | 2024-01-23 | Google-maintained, tiny surface, stable API. Low concern. |
| `exaring/otelpgx` | v0.11.1 | 2026-05-21 | Actively released, but a small single-vendor bridge in the tracing path. Bus-factor risk rather than staleness. |

### Behind but healthy — upgrade candidates for the next cycle, not this RC

All actively maintained; we are simply not on the newest version. Deliberately **not**
upgraded, to keep the RC diff to security-necessary changes only.

`golang.org/x/crypto` v0.52.0 → v0.54.0 · `rs/zerolog` v1.33.0 → v1.35.1 ·
`jackc/pgx/v5` v5.9.2 → v5.10.0 · `prometheus/client_golang` v1.22.0 → v1.24.1 ·
`redis/go-redis/v9` v9.21.0 → v9.22.0 · OpenTelemetry v1.44.0 → v1.45.0 ·
`aws-sdk-go-v2/service/s3` v1.101.0 → v1.106.4

`golang.org/x/crypto` is the one to take first — it is auth-adjacent, and the project's
own policy already forbids auto-merging it, so it needs a deliberate reviewed bump.

### Known-unreachable Go advisories — informational, no action

Both are reported by govulncheck as present but not called, so the CI gate correctly
stays green.

- **GO-2026-5942** (CVE-2026-46600) — `golang.org/x/net` v0.55.0, panic parsing invalid
  SVCB/HTTPS DNS records. Fixed in v0.56.0. We never call `dns/dnsmessage`. A free bump
  whenever `x/net` moves for another reason.
- **GO-2026-5932** — `golang.org/x/crypto/openpgp` is unmaintained and unsafe by design.
  **Fixed in: N/A — no fix will ever exist.** We do not use `openpgp`.

That second one is the clearest argument for reachability-gating rather than
severity-gating: a severity gate would either block this build forever or require a
permanent allowlist entry that then hides future `x/crypto` findings.

### Frontend transitive advisories — monitor, do not force

Twelve production advisories remain, all transitive, nearly all arriving through
`@opennextjs/cloudflare` and the build/adapter chain rather than served request-path code:
`sharp` (libvips CVE-2026-33327/33328/35590/35591), `postcss`, `hono`, `@hono/node-server`,
`js-yaml`, `brace-expansion`, `fast-uri`, `ip-address`, `@babel/core`, `body-parser`,
`@modelcontextprotocol/sdk`.

`npm audit fix --force` resolves them by installing **next 16** — a major version bump of
the frontend framework, days before a soak. That trade is clearly wrong. The project's own
dependency policy already forbids auto-merging framework majors.

**Recommendation:** leave them, and close them out after the soak by moving to next 16 as
a planned, tested upgrade. Two worth checking specifically at that point:

- **`sharp`/libvips** — four CVEs, but it is plausibly never invoked at runtime on
  Cloudflare Workers. Confirming that would downgrade this from "monitor" to "not
  applicable"; it was not investigated here.
- **`hono`** — an HTTP router in the Workers adapter, so unlike the build tooling it may
  genuinely sit in the request path.

**Gap:** npm dependencies are not scanned in CI at all. `govulncheck` is Go-only, and
`npm audit` runs only by hand. Dependabot covers upgrades but is not a build gate.

## Part 4 — Risk assessment

**Risk of the changes made: low.**

- Three Go modules moved: two indirect security patches and one forced transitive minor.
  No direct dependency that this code calls changed its API.
- Next.js moved a patch within 15.5.x, and lockfile churn stayed inside `next`/`@next/*`.
- The full local battery passes, including the integration suite that did not exist in CI
  a session ago.

**Risk of the changes not made: accepted and bounded.** The unreached Go advisories cannot
execute. The frontend transitive advisories sit in build tooling, and the alternative — a
framework major immediately pre-soak — carries materially more risk than the advisories do.

**The residual risk worth naming:** `setup-go`'s `"1.26"` and the Dockerfile's
`golang:1.26-alpine` resolve independently. They agree today at go1.26.5, so the scan
currently describes the artifact accurately. Nothing enforces that. If they ever diverge,
the gate keeps passing while quietly describing a binary that is not the one being
shipped. Deriving both from a single pinned patch is the correct fix and is not yet done.

**Unchanged from the previous report:** Playwright still does not execute in CI, so 387
end-to-end tests remain unrun. That is the largest gap in this release, and it is not a
dependency problem.
