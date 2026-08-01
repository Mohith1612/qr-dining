# Post-Remediation Rollout Status

Date: 2026-05-26
Supersedes the readiness snapshot in `final-rollout-gates-status.md` with the
post-remediation picture. Status: **GO to begin R1. R1 not yet started.**

---

## 1. Where we are

Phases A–E complete; pre-R1 Playwright sweep surfaced 43 failures; the remediation pass
(`rollout-blocker-remediation-report.md`) fixed all **5 genuine P0 blockers** and
committed them:

| ID | Blocker | Fix (committed) | Verified live |
|----|---------|-----------------|---------------|
| T-01 | Cross-org snapshot leak (security) | Reject present-but-invalid guest token (no fail-open) | 200 → 401 |
| X-03 | Staff token on guest route → 500 | same root | 500 → 401 |
| O-05 | Zero-quantity order → 500 | per-item quantity validation | 500 → 400 |
| P-09 | Overpayment accepted | reject `amount > bill.Total` (422) | 201 → 422 |
| R-06 | Snapshot authority ambiguity | `snapshot_authoritative` signal | added |

The remaining 38 failures are spec/helper/env/frontend drift — **not** rollout blockers
(classified in `non-blocking-e2e-cleanup-plan.md`).

---

## 2. Wave re-evaluation (R1 → R7) — conservative, no acceleration

> The dependency order and soak periods from `production-enforcement-rollout.md` stand
> unchanged. Remediation did **not** shorten any soak or skip any gate.

### R1 — `AUDIT_LOG_V2_ENABLED` — **READY, flip first**
- Blockers: none. Instrumentation + alert (`AuditWriteFailures`) + dashboard in place.
- Remaining: disk-headroom check + 48 h staging / 72 h prod soak (operational, not code).
- See `r1-activation-runbook.md`.

### R2 — `TENANCY_ORGANIZATIONS_ENABLED` — ready after R1 soak
- Blockers: none code. `tenant_resolution_failures_total{stage}` +
  `TenantResolutionFailures` alert exist (Phase E).
- Remaining: confirm all `branches.organization_id` NOT NULL in prod; soak to zero
  resolution failures. Starts only after R1's 72 h clears.

### R3 — `AUTHZ_CENTRAL_POLICY_ENFORCE` + `STRICT_BRANCH_SCOPED_MUTATIONS` (paired) — **policy blocker**
- Instrumentation ready: `policy_shadow_mismatch_total{route,reason}`,
  `authz_denied_total{reason}`, alerts `PolicyShadowMismatchPresent` / `AuthzDeniedSpike`.
- **Hard blocker:** the three §11 open policy decisions in
  `production-enforcement-rollout.md` must be answered **in writing** before R3 (org-owner
  mutation rights; revoked-credential assistance; org audit aggregation).
- Remaining: 7-day shadow week driving `policy_shadow_mismatch_total` to zero (48 h
  consecutive). Relevant to the T-01 area — cross-org enforcement now correct at the guest
  layer; R3 extends central enforcement to staff actions.

### R4 — `AUTH_STAFF_CODE_REQUIRED` + `AUTH_STAFF_SESSION_DB_REQUIRED` (paired) — data/training gated
- Blockers: none code. Remaining: every active staff has a `staff_code`; trained on the
  new login during shadow week; `legacy_identity_usage_total{mechanism="staff_branch_pin"}`
  zero for 7 days. Biggest support risk on flip day.

### R5 — `WS_TICKET_AUTH_REQUIRED` — sizing gated
- Instrumentation ready: `ws_ticket_consume_failed_total{reason}` +
  `WSTicketConsumeFailures` alert; WS inbound abuse hardening shipped (Phase D).
- Remaining: drive `legacy_identity_usage_total{mechanism="ws_query_participant_id"}` to
  zero (7 d); **size the per-IP ws-ticket cap (60/min)** against real Cloudflare/NAT
  egress before flip (Phase D chaos §4 — mass post-deploy reconnects behind one egress IP).

### R6 — `AUTH_GUEST_CREDENTIALS_REQUIRED` — legacy-decay gated
- Instrumentation ready: `guest_token_validation_failed_total{reason}` +
  `GuestTokenValidationFailures` alert.
- **Note from remediation:** the T-01 fix already rejects *present-but-invalid* guest
  tokens regardless of this flag. R6 additionally rejects the *no-token / legacy
  participant-id* path. Remaining: legacy mechanisms zero for 14 d; 14 d prod soak.

### R7 — `PAYMENT_STAFF_SETTLEMENT_REQUIRED` — workflow gated
- Instrumentation ready: alert-only `payment_pending_escalations_total{level}` +
  `PaymentPendingEscalationCritical` (Phase E). P-09 overpayment bound now enforced.
- Remaining: staff settlement UI on every branch device; settlement-latency soak;
  out-of-hours workflow; webhook idempotency integration test green in CI (committed, see
  Phase E caveat). Highest operational risk; last.

---

## 3. Final pre-R1 confidence assessment (calm and honest)

**What is strong**
- R1 itself is genuinely low-risk: write-path only, non-fatal writer, instant reversible
  rollback (flag false), zero data risk, immutable trigger enforced at the DB.
- The integrity signal (`audit_write_failures_total`) is wired to a page and a dashboard;
  operators can see failure within minutes.
- The 5 P0 blockers — including the cross-org leak and the money-correctness bug — are
  fixed and verified live, not just asserted.

**What still worries me**
- **Overconfidence after the fixes.** The biggest risk now is human: treating "P0s fixed"
  as license to chain waves. R2–R7 each retain real soak/operational gates. R3 has a hard
  *policy* blocker that no amount of engineering clears.
- **E2E suite is not a green gate yet.** 38 drift failures remain and the suite needs a
  platform-admin bootstrap to run in CI. Until cleaned up, "the suite is red" must not be
  read as "the product is broken" — nor should a green subset be read as full coverage.
- **Storage growth is the quiet R1 risk.** Audit volume can surprise; the projection must
  be tracked, not assumed.

**What could realistically fail during R1**
- A sustained `audit_write_failures_total` from an unexpected DB contention or a metadata
  blob that trips a constraint → audit gaps. Mitigation: the page + rollback.
- Latency creep on hot write routes under peak if the extra INSERT contends for pool
  connections. Mitigation: p99 watch + pool metrics + rollback.

**What operators must watch carefully**
- `audit_write_failures_total` (primary), audited-route p99, `audit_log` row growth vs
  disk free. See `r1-soak-monitoring-guide.md`.

**What is safe to defer (post-launch / non-blocking)**
- The 38 drift failures (separate cleanup pass).
- Tuning escalation thresholds beyond conservative defaults.
- Frontend adoption of the `PAYMENT_SETTLEMENT_STALLED` event.
- Legacy code-path removal (already 30-day gated).

---

## 4. Recommendation

**Proceed with R1** using `r1-activation-runbook.md` + `r1-soak-monitoring-guide.md`.
After R1's 72 h production soak clears cleanly, consider R2 on its own merits. Do not
pre-stage or chain later waves. Resolve the R3 policy questions in writing well before R3.
