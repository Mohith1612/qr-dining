# Release

The release lifecycle, its gates, and everything still open before v1.0.

Status as of **2026-08-22**, verified against the working tree at `feature/signoz-observability` @ `9865a48`.

Companions: [TESTING.md](TESTING.md) · [DEPLOYMENT.md](DEPLOYMENT.md) · [OPERATIONS.md](OPERATIONS.md) · [../STATE-OF-THE-PROJECT.md](../STATE-OF-THE-PROJECT.md).

---

## 1. Branch model (current reality, not aspiration)

| Branch | Position | Meaning |
|---|---|---|
| `main` @ `2ac5ba3` | — | **Stale spine.** Ends at the R3 governance regression tests, 205 commits behind reality |
| `feature/signoz-observability` @ `9865a48` | +205 / −0 vs `main` | **The de-facto trunk and RC.** Strict superset of every other branch |
| `feature/certification-fixes-ui-redesign` | absorbed | Ancestor of the trunk. Prunable |
| `feature/staff-analytics-loyalty` | absorbed | Ancestor of the trunk. Prunable |
| `premium-qr-collateral` | absorbed | Ancestor of the trunk. Prunable |
| `pilot-readiness-remediation` | absorbed | Ancestor of the trunk. Prunable |
| `platform-governance-entitlements` | absorbed | Ancestor of the trunk. Prunable |

`origin` is `git@github.com:Mohith1612/qr-dining.git`. **PR #1** is open against `main` with head `b57f746`, which is four commits behind local HEAD. The only tag in the repository is `redesign-foundation`; **no release tag exists.**

Target model after the first merge: `main` is the only long-lived branch, short-lived feature branches merge back promptly, and the flag system — not branches — is the isolation mechanism.

## 2. Release sequence

```
manual certification (human gate)
  → freeze RC
  → merge to main --no-ff
  → green CI on main
  → tag v1.0.0-rc.1        ← this is what publishes the GHCR image
  → build soak binary FROM THE TAG
  → multi-day RC soak       ← SEV-0
  → production provisioning
  → first restaurant
```

Each arrow is a gate, not a suggestion. In particular the soak binary is built **from the tag**, never from a branch tip, so the artifact that soaked is the artifact that ships.

## 3. CI gates

CI triggers **only** on push to `main`, `v*` tags, PRs into `main`, and `workflow_dispatch`. A feature branch that is never PR'd gets no CI. This is why the RC branch sat 190 commits deep with "CI is green" as an untested assumption until PR #1 forced the first real run.

Backend (`ci.yml`): lint (gofmt/vet/golangci-lint v2.12.2) · build + unit tests with race · govulncheck · sqlc drift · migrations up/down/up · **integration tests with race against real Postgres+Redis** · OpenAPI lint · arm64 Docker build, pushed to GHCR on `main` and `v*`.

Frontend (`frontend-ci.yml`): frontend lint + build · marketing build · Playwright **discovery only**.

Full detail, and what CI does *not* cover, is in [TESTING.md §5](TESTING.md#5-what-ci-actually-runs). The two that matter most for a release decision:

- **Playwright is never executed in CI.** Run the suite locally before any gate.
- **OpenAPI lint proves validity, not route agreement.** The spec can be valid and still drift from the server.

## 4. Manual certification

The last human gate before the RC freezes. Materials and process: [TESTING.md §6](TESTING.md#6-manual-certification).

**Current run — 2026-08-22 — is `BLOCKED`.** Exploratory functional testing may proceed; formal sign-off may not. The blockers, from [../release-certification/manual-certification-preflight-2026-08-22.md](../release-certification/manual-certification-preflight-2026-08-22.md):

1. **No immutable candidate.** The certification checkout is `9865a48`; both beta backends run `qr-dining:beta-b57f746` built from `b57f746`; the deployed frontend artifact carries no commit label at all. Three different things are being called "the candidate".
2. **SigNoz ingestion is down.** ClickHouse is exceeding its 1.35 GiB limit and the collector is dropping logs, traces and metrics.
3. **Nine reachable advisories** in the exact deployed backend binary (Go 1.26.5).
4. **Alertmanager routes to a local sink**, not to a human.

None of these are product defects. All four are release-engineering gaps.

## 5. Open before v1.0

### SEV-0

- [ ] **Fresh multi-day soak of the tagged build.** The 124-hour R1 soak that passed on 2026-06-04 ran the pre-redesign June binary. The certification fixes, migrations 000036–000039, the Serene/Harmony redesign, PostHog analytics and the OTel work are all unsoaked. Build from the tag, host the binary **off `/tmp`** (tmp-cleaner has wiped it twice), `AUDIT_LOG_V2_ENABLED=true`, continuous traffic plus external probing, record the storage growth curve.

### SEV-1

- [ ] Merge the trunk to `main` with `--no-ff`, confirm CI green on `main`, tag `v1.0.0-rc.1`.
- [ ] Wire a **real Alertmanager receiver** plus an app-down/`/readyz` page alert. One line: the `&notify_url` anchor.
- [ ] Name one immutable candidate and deploy exactly it to beta — backend *and* frontend, both provenance-labelled.
- [ ] Fix SigNoz/ClickHouse memory so the ingestion path works, or explicitly de-scope tracing from the certification.
- [ ] Resolve or accept the nine reachable advisories in the deployed binary.
- [ ] `proxy_certbot` has been `Exited (137)` for months and its entrypoint runs `certbot renew --webroot`, which cannot renew the wildcard cert this deployment uses. Renewal is effectively manual. Current cert expires **2026-10-27**.

### Production infrastructure

- [ ] Provision the production host (Oracle Ampere arm64) per [DEPLOYMENT.md](DEPLOYMENT.md).
- [ ] Purchase the domain; replace every `CHANGEME-DOMAIN.com` per [DEPLOYMENT.md §8](DEPLOYMENT.md#8-go-live-substitution-list-when-domainbuckets-are-final).
- [ ] Move off the temporary `qr-dining-backups` / `qr-dining-uploads` buckets.
- [ ] Schedule nightly R2 backups against the production bucket and wire the backup textfile metrics into node-exporter.
- [ ] Exercise the OpenNext → Cloudflare production deploy once end-to-end with real env vars and no localhost bypass.
- [ ] Firewall: only 80/443 public; 9090/9093 private; Postgres and Redis never published.

### Accepted for the pilot, must fix before scale

- [ ] Money math uses floats — convert to integer paise.
- [ ] `session_sequences` write hot-spot (~5% 5xx at concurrency ~150; clean at 50).
- [ ] Frontend `no-unused-vars` warnings and dead scaffolding.
- [ ] Real payment gateway integration — the pilot is cash/UPI-manual with simulated webhooks.
- [ ] Webhook exact-replay proof in CI. This gates rollout wave R7.
- [ ] An e2e runbook: the suite needs a live stack, a real `E2E_ADMIN_TOKEN` from `POST /platform/auth`, and raised `AUTH_RATE_LIMIT_RPM`/`RATE_LIMIT_RPM` — the default 10 RPM auth limit rate-limits the suite into mass failure.

### Housekeeping after the merge

- [ ] Delete the five fully-absorbed branches.
- [ ] Ensure the RC tag contains its own documentation — several operational docs were untracked at earlier tag attempts.

## 6. Rollout waves

Enforcement is staged behind nine environment flags rather than branches. The philosophy: **shadow before strict wherever legacy traffic or data state exists; metrics gate data migrations; flags are reversible in under five minutes; humans stay the authority for money.**

Current posture, and the procedure for changing it, is in [OPERATIONS.md §4](OPERATIONS.md#4-rollout-flags-the-enforcement-ladder). R4–R6 ship on as the launch baseline because there is no legacy client population — the only frontend already speaks staff codes, DB-backed sessions, WS tickets and signed guest credentials.

## 7. Beta versus production

They are different environments and the distinction is load-bearing.

|  | Beta | Production |
|---|---|---|
| Exists | **Yes** — live now | **No** — not provisioned |
| Hostnames | `qr-beta.mohith16.com`, `qr-api-beta*.mohith16.com` | Domain not purchased |
| Data | Seeded demo tenants | — |
| Backend | Built on the VM from a branch, `qr-dining:beta-<sha>` | Will be a GHCR image from a `v*` tag |
| Alerting | Local sink container | Requires a real receiver |
| R2 | One shared credential pair, temporary buckets | Separate scoped tokens, final buckets |
| Purpose | Manual certification on real devices over real TLS | Paying customers |

Nothing has been deployed to production. No claim in this repository should be read as saying otherwise.

## 8. Certification evidence

Historical rounds are preserved under [../release-certification/](../release-certification/) — see its README for what is current and what is archived.
