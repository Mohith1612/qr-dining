# Final Pilot Readiness Report

**Date:** 2026-06-10
**Branch:** `premium-qr-collateral` (all hardening below committed here)
**Scope:** operational hardening only — no product features, no architecture/lifecycle/
rollout changes — closing the operational SEV-0/SEV-1 findings from the pre-pilot audit.

## What this phase changed (all committed, each verified)

| Phase | Change | Commit | Verification |
|-------|--------|--------|--------------|
| A | Release mode fails hard on insecure secret/origin defaults (dev `GUEST_TOKEN_SECRET`, weak length, empty CORS, malformed MFA/webhook secrets) | `f91383e` | 7 unit tests + live boot: dev secret & empty CORS → fatal; secure → boots |
| B | Production frontend build blocked on localhost/insecure endpoints (`prebuild` guard) | `0637b6e` | 6 cases incl. `npm run build` blocked before `next build` |
| C | Container healthchecks point at `/readyz` (readiness) not `/health` | `a2a3891` | live: `/readyz` 200→503 on Redis down while `/health` stays 200 |
| D | Automated nightly backup (R2/S3/local provider abstraction, manifest, retention) + scheduling units | `ebb4b16` | full pipeline run; 2-run manifest append; valid pg dump |
| E | Real restore verification on a throwaway DB | `463950c` | 56/56 tables, row-for-row identical, sentinel + trigger fidelity, checksum match |
| F | Alertmanager routing + `ReadyzProbeFailing`/`Backup*` alerts + runnable observability stack | `a90a96f` | `promtool`/`amtool` OK; live alert delivered to a receiver |
| G | Synthetic load driver + pilot load validation | `81d60c7` | concurrency 50: p99 ~200ms, ~0.4% genuine errors, pool ample; ceiling characterized |

## Required-condition coverage (the four operational gaps)

- **Secrets safe in prod:** ✅ fail-hard in release (A).
- **Frontend can't ship localhost:** ✅ build-time guard (B).
- **Readiness vs liveness probes correct:** ✅ (C).
- **Automated, *verified* backups:** ✅ nightly + real restore (D, E).
- **Alerts actually notify (app/DB/Redis/backup down):** ✅ wired + demonstrated (F).
- **Capacity proven at pilot scale:** ✅ (G).

---

## Remaining SEV-0 (must close before live service)

1. **Run a soak on the pilot code.** The pre-pilot audit's #1 blocker — *the soaked
   binary ≠ the pilot binary* — is now **resolvable, not yet resolved**: all hardening
   is committed to `premium-qr-collateral`, so a fresh soak on **this branch** would
   finally soak the real pilot code (incl. the unmerged platform/billing/theme/collateral
   layer and the session-join behavior change). **Action:** restart the soak on this
   branch with `AUDIT_LOG_V2_ENABLED=true` and let it run its full window before the
   first paying dinner. Until that soak passes, this stays SEV-0.

*(The other original SEV-0 — warn-only `GUEST_TOKEN_SECRET` — is CLOSED by Phase A.)*

## Remaining SEV-1 (close before first paying customer)

1. **Real R2 round-trip + real alert receiver.** Backups and alerting are built and
   verified with a local provider / webhook sink. Before go-live: run one
   `nightly-backup.sh` against the **production R2 bucket**, restore from the downloaded
   object, and point Alertmanager at a **real** channel (Slack/PagerDuty/email) with one
   test alert. Code is done; only the live credential exercise remains.
2. **Authorization is still shadow-mode by default** (`AUTHZ_CENTRAL_POLICY_ENFORCE=false`;
   R3 unflipped). Out of scope here (rollout sequencing must not change). Acceptable for a
   pilot where all staff are trusted employees, but intra-branch role boundaries rely on
   per-handler checks, not central enforcement. Flip via the R-wave process post-pilot.

## Remaining SEV-2 (acceptable for pilot; address after)

1. **Keep pilot billing manual** — subscription/invoicing (migration 032) is new and
   shadow-only; do not charge a pilot through it.
2. **Single-branch write hot-spot** — under stress (≈150 continuous single-branch
   writers) the per-branch `session_sequences` row serializes and a small fraction of
   writes 500 (p99 >1s). Far beyond pilot load; investigate before high-volume scale.
   Raise `DB_MAX_CONNS` to ~40–50 and give Postgres its own host at scale.
3. **Order placement isn't audited**; **multi-instance WS** is unproven; **money math uses
   float accumulation**. All documented in the audit; none block a supervised pilot.

Closed since this report: release mode ignores dotenv files; reconnect exhaustion
offers an in-place Retry action; and promo daily windows use branch-local time.

## SEV-3

`/readyz` couples Redis to readiness (by design — a Redis-down app genuinely can't serve);
60-min terminal read window; no audit hash-chain; Hub room map unbounded. Defer.

---

## GO / NO-GO

**GO for a single, closely-supervised restaurant pilot — conditional on the one remaining
SEV-0: a clean soak of *this* branch.**

The operational gaps that made the pre-pilot audit a NO-GO are now closed and verified:
secrets fail-hard, the frontend can't ship localhost, healthchecks drain unready
instances, backups are automated **and** restore-verified, alerts actually page, and
capacity is proven comfortable at pilot scale. The engineering core was already strong;
this phase made the operations trustworthy.

The honest gate: **do not serve the first paying dinner until (1) a fresh soak on
`premium-qr-collateral` passes its window, and (2) the live R2 backup round-trip + a real
alert receiver are confirmed.** Keep billing manual and rely on trusted-staff for the
shadow-mode authz. With those, this is a defensible, well-instrumented controlled pilot.

**Verdict: GO (conditional), pending the pilot-branch soak.**
