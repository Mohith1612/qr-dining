# Final Rollout Gates Status

Date: 2026-05-25 (post Phase E)
Reads against `production-enforcement-rollout.md` (wave defs) and
`strict-rollout-plan-final.md` (Phase-D gating). This is the per-wave readiness ledger
after Phase E closed the instrumentation gaps.

Legend: ✅ ready · ⚠️ needs soak tuning / operational step · ⛔ blocker remains.

## R1 — AUDIT_LOG_V2_ENABLED
- Observability: ✅ `audit_write_failures_total` + `AuditWriteFailures` alert (pre-existing).
- Rollback: ✅ flag false, no data risk.
- Remaining: ⚠️ disk-headroom check + 72h prod soak. **No code blocker.**
- Status: **READY to flip first.**

## R2 — TENANCY_ORGANIZATIONS_ENABLED
- Observability: ✅ `tenant_resolution_failures_total{stage}` + `TenantResolutionFailures`
  alert + dashboard panel (Phase E).
- Rollback: ✅ flag false.
- Remaining: ⚠️ confirm all `branches.organization_id` NOT NULL in prod; soak to zero
  resolution failures.
- Status: **READY** (instrumentation prerequisite met).

## R3 — AUTHZ_CENTRAL_POLICY_ENFORCE + STRICT_BRANCH_SCOPED_MUTATIONS (paired)
- Observability: ✅ `policy_shadow_mismatch_total{route,reason}` (the would-break signal),
  `authz_denied_total{reason}`, alerts `PolicyShadowMismatchPresent` / `AuthzDeniedSpike`,
  dashboard panel (Phase E). The shadow counter is exactly what the wave's "mismatch rate
  zero for 48h" gate needs.
- Rollback: ✅ both false together.
- Remaining: ⛔ the three §11 open policy decisions must be answered in writing before R3
  (org-owner-without-staff mutation rights; revoked-credential assistance; org audit
  aggregation). ⚠️ run the shadow week and drive `policy_shadow_mismatch_total` to zero.
- Status: **INSTRUMENTATION READY; policy decisions outstanding (⛔).**

## R4 — AUTH_STAFF_CODE_REQUIRED + AUTH_STAFF_SESSION_DB_REQUIRED (paired)
- Observability: ✅ `authz_denied_total`; staff login uses existing
  `staff.login.failed` audit + `legacy_identity_usage_total{mechanism="staff_branch_pin"}`.
- Rollback: ✅ both false; existing `staff_sessions` reusable.
- Remaining: ⚠️ data backfill (`staff_code` for every active staff), staff trained via new
  flow during shadow week, legacy PIN usage decayed to zero 7d. Operational, not code.
- Status: **READY pending staff data/training soak.**

## R5 — WS_TICKET_AUTH_REQUIRED
- Observability: ✅ `ws_ticket_consume_failed_total{reason}` + `WSTicketConsumeFailures`
  alert + panel (Phase E). WS inbound abuse hardening shipped Phase D.
- Rollback: ✅ flag false; open sockets unaffected.
- Remaining: ⚠️ `legacy_identity_usage_total{mechanism="ws_query_participant_id"}` zero 7d;
  size the **per-IP** ws-ticket cap (60/min) against real Cloudflare/NAT egress before flip
  (Phase D chaos §4 — mass post-deploy reconnects behind one egress IP).
- Status: **READY pending per-IP cap sizing + legacy decay.**

## R6 — AUTH_GUEST_CREDENTIALS_REQUIRED
- Observability: ✅ `guest_token_validation_failed_total{reason}` +
  `GuestTokenValidationFailures` alert + panel (Phase E). Reason breakdown lets ops confirm
  failures are expired/invalid (expected) not malformed (stale clients).
- Rollback: ✅ flag false; existing tokens still accepted.
- Remaining: ⚠️ legacy participant-id mechanisms zero for 14d; 14d prod soak.
- Status: **READY pending legacy decay soak.**

## R7 — PAYMENT_STAFF_SETTLEMENT_REQUIRED
- Observability: ✅ `payment_pending_escalations_total{level}` +
  `PaymentPendingEscalationCritical` page + panel + alert-only escalation worker (Phase E).
  This is the settlement-age visibility the wave required.
- Rollback: ✅ flag false; pending payments unaffected.
- Remaining: ⚠️ staff settlement UI on every branch device; settlement-latency soak; webhook
  idempotency integration test green in CI (committed; see report caveat). Out-of-hours
  settlement workflow documented.
- Status: **READY pending staff UI rollout + CI test run + soak.**

## Summary

| Wave | Code/instrumentation | Operational/soak | Hard blocker |
|------|----------------------|------------------|--------------|
| R1 | ✅ | disk + 72h | — |
| R2 | ✅ | org NOT NULL + soak | — |
| R3 | ✅ | shadow week | ⛔ §11 policy decisions |
| R4 | ✅ | staff backfill/training | — |
| R5 | ✅ | per-IP cap sizing | — |
| R6 | ✅ | 14d legacy decay | — |
| R7 | ✅ | staff UI + CI test | — |

**Every instrumentation prerequisite is now met.** The only remaining *hard* blocker is
the R3 policy decisions (a writing task, not code). Everything else is soak time and
operational rollout (staff data, UI, cap sizing) — none of which blocks *starting* the
sequence at R1.
