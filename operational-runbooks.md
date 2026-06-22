# Operational Runbooks

Date: 2026-05-22
Status: Authoritative incident playbooks. Updated whenever architecture changes. Every on-call engineer must have read this end-to-end before taking pager.

## 0. Conventions

- **Severity 1 (Sev1)**: customer-visible total outage, money-handling impacted, or data loss risk. Page all on-call.
- **Severity 2 (Sev2)**: degraded service, partial outage, single branch impacted. Page primary on-call.
- **Severity 3 (Sev3)**: latent issue, not impacting active service.

Every runbook follows the same shape:
1. Detection signals
2. Immediate triage
3. Containment
4. Recovery
5. Verification
6. Post-incident actions

## 1. Auth Outage (staff or guest)

### Detection signals

- `staff.login.failed` rate > baseline x 5 over 5 minutes.
- `guest.token.validation_failed` rate > baseline x 5.
- `staff_sessions` insert errors in app logs.
- DB connection saturation on `staff_sessions` or `session_participants`.

### Immediate triage

- Check DB health. If DB write rejects across the board, this is a DB incident, not auth (jump to DB runbook).
- Check `GUEST_TOKEN_SECRET` and `STAFF_TOKEN_SECRET` env vars on running pods — a recent deploy may have rotated incorrectly.
- Check `audit_log` insert rate: a backed-up audit writer can block requests when `AUDIT_LOG_V2_ENABLED=true` if writer is synchronous (it should not be, but verify).

### Containment

- If the auth-only path is broken and `AUTH_GUEST_CREDENTIALS_REQUIRED` is strict, consider flipping back to permissive for the affected wave. This is a Sev1 decision and requires the incident commander's approval.
- Drain the bad pods if errors are pod-local.

### Recovery

- Restore the previous deploy if the regression correlates with deploy time.
- Re-apply secrets if a rotation was the cause.
- Restart pods to re-establish DB connection pools.

### Verification

- `staff.login.success` rate returns to baseline.
- A test staff login from an internal device succeeds.
- A test guest QR scan + join + cart action succeeds.

### Post-incident

- Write incident report including timeline, blast radius (how many sessions affected), recovery steps, RCA.
- File tickets for any process gap.

## 2. Redis Outage

### Detection signals

- `redis_client_errors_total` rate spikes.
- WS clients disconnecting en masse.
- Rate limiter unable to read counts.
- Presence keys missing.

### Immediate triage

- Confirm Redis health from the network: ICMP, TCP, AUTH probe.
- Distinguish: full outage, partial (one shard), or latency.

### Containment

- Frontend automatically degrades WS to snapshot-polling (see Section 12 of `realtime-reconciliation-invariants.md`). Verify.
- If rate limiter fails open for non-critical surfaces and fails closed for critical, verify that critical (staff auth, payment, webhook) is correctly fail-closed.
- For Redis-backed WS tickets, clients reconnect by fetching new tickets; that endpoint must remain available.

### Recovery

- Restart Redis or failover to replica.
- Once Redis is healthy, app instances resume publishing. No manual intervention needed.

### Verification

- WS clients reconnect and resume receiving live events.
- Rate-limit dashboards show normal counts.
- `realtime_publisher_lag_seconds` p95 < 1s.

### Post-incident

- If outage lasted > 5 minutes, audit whether any state inadvertently relied on Redis as source of truth. Should be none; document if found.
- Confirm presence keys rebuilt by next heartbeat cycle.

## 3. WebSocket Degradation

### Detection signals

- WS connect attempts succeeding but client-reported "live updates not arriving."
- `realtime_publisher_lag_seconds` p95 elevated.
- `ws_outbound_buffer_overflow_total` elevated.

### Immediate triage

- Pull a sample session and trace event flow: DB insert -> publish -> hub broadcast -> client receive.
- Check Redis pubsub lag.

### Containment

- If a specific app instance is unhealthy, drain it. Clients reconnect to healthy peers.
- If pubsub is slow but functional, frontend reconciliation (via snapshot refresh on focus) keeps users mostly correct.

### Recovery

- Restart unhealthy app instances.
- If pubsub topic backed up across the cluster, restart Redis (treat as Section 2).

### Verification

- Sample session: a status change on one device appears on another within 2 seconds.

### Post-incident

- Confirm clients did not get stuck on stale sequence numbers. If reports of "I have to refresh to see updates," ensure DEL-4 / SNP-1 are working.

## 4. Stuck Payments

### Detection signals

- `payment_pending_age_seconds` p95 exceeds 10 minutes.
- `payment.stuck` audit events being written.
- Staff support tickets about "tap to pay did nothing."

### Immediate triage

- Identify the stuck payment row. Note method (cash/card_manual/digital).
- For digital: check webhook backlog (Section 5) and provider status page.
- For cash/manual: confirm staff workflow is reachable (settle button works).

### Containment

- If a single session: instruct staff to use the `cancel_pending_payments` action (see `payment-finalization-invariants.md` TIM-2). Session returns to `active`. Guest can retry.
- If many sessions across branches: this is likely a provider outage or a webhook backlog (Section 5).

### Recovery

- For provider outage: switch the affected provider to "manual settlement" mode via flag, instruct branches to settle as cash.
- For webhook backlog: process backlog manually via the reprocess CLI.

### Verification

- `payment_pending_age_seconds` p95 returns to baseline.
- No new `payment.stuck` audit events.

### Post-incident

- Reconcile completed payments against provider statements for the outage window.

## 5. Webhook Outage

### Detection signals

- `payment_webhook_events.processed=false` rows older than 30 min.
- `payment.webhook.signature_failed` rate spike.
- Provider dashboard shows our endpoint as failing.

### Immediate triage

- Verify our endpoint health: send a known-good signed test event.
- Verify our signature secret has not been rotated incorrectly. Compare against provider dashboard.
- Check `cfg.Payment.WebhookSecrets` for the affected provider on running pods.

### Containment

- If signature secret mismatch: redeploy with the corrected secret. Provider will retry within their backoff.
- If our endpoint is down: restore. Provider will retry.
- Inform branches that digital settlement may be delayed; staff should accept fallback methods.

### Recovery

- Once endpoint healthy, provider retries land. `processed=true` rate climbs.
- For any events that timed out beyond provider retry policy, run the reprocess CLI with the event id from the provider dashboard.

### Verification

- `payment_webhook_events.processed=false` rate returns to zero for events older than 30 min.
- Reconciliation report (daily) clean for the affected day.

### Post-incident

- Update on-call playbook with provider-specific retry SLA.
- Confirm rotation procedure does not produce same incident.

## 6. Feature Flag Rollback

### When

- Any production alert tied to a recently-flipped flag.
- Any support escalation tied to behavior the flag introduced.

### Steps

1. Incident commander declares rollback.
2. Update environment variable in deployment manifest: flag from `true` to `false`.
3. Apply with rolling restart. Confirm via:
   - `/healthz` endpoint reflecting flag state if exposed.
   - Behavioral test (e.g., for `AUTH_STAFF_CODE_REQUIRED`, a legacy `branch_id+PIN` login should succeed).
4. Watch error metrics for 30 minutes. They should fall to pre-flip baseline.
5. File post-incident: what triggered the flip, what triggered the rollback, what is the remediation before re-flipping.

### Constraints

- A flag rollback never accompanies a code change in the same deploy.
- A flag rollback never accompanies a schema change in the same deploy.

## 7. Migration Rollback

### When

- A migration left data in an inconsistent state.
- A migration broke an existing query.

### Constraints

- Every migration in `backend/migrations/` has a `.down.sql` counterpart. Verify the down migration is safe (no destructive data drops on rollback) before applying.
- Migrations 000016-000022 are additive. Rollback is generally safe.
- Constraint-adding migrations (e.g., partial unique indexes) can be rolled back by dropping the index.

### Steps

1. Determine the last good migration version.
2. Apply `migrate down` to that version on the impacted environment.
3. Restart pods so the application matches the rolled-back schema (if a deploy paired the migration with code changes, redeploy the previous code).
4. Verify functional smoke test.

### Caveats

- Strict-mode flags should be rolled back before the migration to avoid runtime errors against an older schema.
- Any data created since the migration that depends on new columns may be lost on `down`. Snapshot the DB before rollback.

## 8. Stale Session Recovery

### When

- A branch reports "table X shows occupied but nobody is there."
- A guest reports "I can't start a new session, it says table is occupied."

### Triage

- Pull `sessions WHERE table_id = $T AND status IN ('active','payment_pending','awaiting_reactivation')`.
- Compare against table status.
- If a non-terminal session exists with stale `last_activity_at`, the reconciliation worker should have moved it. Check `worker_ticks` and worker logs.

### Containment

- Staff has a manual `force_close` action behind authz. Use it. The action transitions the session to `closed` (with `reason='staff_force_close'`) and audits.
- If `force_close` is not yet exposed, run SQL transaction: update session to `closed`, update table to `available`, revoke participants, all in one transaction. Document this as a manual procedure.

### Recovery

- Guest can scan QR and start a new session immediately.

### Post-incident

- If the worker is not catching this class of stale state, file a ticket and tune `quiet_grace_seconds` or worker tick interval.

## 9. Support Escalation

### When

- A guest claims they paid but session shows unpaid.
- A staff member claims they cannot log in despite correct PIN.
- An organization owner reports data they should see is missing.

### Steps

- Verify the claim against the audit log: actor's recent events, session events, payment rows.
- If the system is correct and the claim is wrong, explain (with `event_reference` / `payment_reference` / `session_number` from operational IDs).
- If the system is wrong, open an incident at the appropriate Sev.

### Escalation paths

- Payment dispute: Operations -> Finance.
- Auth lockout for staff: Branch Manager -> Operations.
- Data missing across an organization: Operations -> Engineering (Sev2 unless tenancy isolation suspected; then Sev1).

## 10. Platform Support Break-Glass

### When

- Operations need to read into a tenant's data to diagnose an issue.

### Procedure

1. Operations engineer requests a support session via `/platform/support/sessions`, body includes `organization_id`, optional `branch_id`, `reason`.
2. Second platform admin approves (`approved_by_platform_user_id`).
3. Session is granted, expires_at set (max 4 hours).
4. Every read or write under that session is audited with `support_session_id`.
5. Tenant-visible audit feed surfaces these entries.

### Restrictions

- Default read-only.
- Write actions require elevated role and an explicit `reason_for_write` field in the request payload.
- No bulk export from a support session — exports go through normal organization channels.

## 11. Webhook Replay Storm

### When

- A provider replays a large batch (e.g., outage recovery from their side).

### Containment

- Idempotency by `external_event_id` handles correctness automatically.
- Rate-limit the webhook surface if pod CPU saturates (already in Section 2 of `security-hardening-checklist.md`).

### Recovery

- Provider naturally throttles after our 200s.

## 12. Long-Running Session Timeout

### Detection

- `session.expired` events spiking at a single timestamp suggest a config change or branch-policy change.
- Customer reports of "I was in the middle of my meal."

### Triage

- Check `branches.session_timeout_minutes` for the affected branch.
- Check whether timeout was changed by an organization admin (`audit.read.organization`).

### Containment

- Restore previous timeout.
- For active sessions in the warning window, a manual extension is possible via staff `extend_session` action (if exposed); otherwise advise staff to monitor the affected tables.

## 13. Branch Suspension

### When

- An organization admin or platform admin suspends a branch (e.g., compliance).

### Effects

- All active sessions in the branch transition to `abandoned` with reason `branch_suspended`.
- All guest credentials revoked.
- Staff cannot log in to the suspended branch.
- Existing platform/org reads of the branch still work.

### Reversal

- Reactivate branch.
- Sessions do not auto-restore; guests must rescan to begin new sessions.
- Outstanding payments are preserved and can be settled by staff after reactivation.

## 14. Daily Reconciliation Failures

### Detection

- Daily reconciliation report (audit `reconciliation.daily`) contains non-zero counts in any critical bucket.

### Triage

- Sessions terminal with non-terminal payments: pull the rows, investigate per Section 4 (Stuck Payments).
- Webhooks unprocessed > 30 min: reprocess via CLI.
- Snapshots `partial` > 24 hours: open dispute workflow.

## 15. Cold Restart Drill (quarterly)

### Steps

1. During lowest-traffic window, restart all backend pods simultaneously.
2. Confirm:
   - All WS clients reconnect within 60s.
   - No payment in flight is lost (all rows still present, status correct).
   - Worker locks release and re-acquire.
   - First batch of `legacy_identity_usage_total` post-restart matches pre-restart baseline.

If any fails, file as Sev2 follow-up.

## 16. Network Partition Drill (quarterly)

### Steps

1. In staging, partition app instances from Redis for 60s.
2. Confirm graceful degradation (Section 2).
3. Restore Redis.
4. Confirm recovery (Section 2 verification).

## 17. Disaster Recovery

- DB backups: daily logical, hourly WAL, 30 days retention. Backup verification monthly.
- Restore drill: quarterly, full restore to staging, verify schema and a sample of business rows.
- RTO target: 1 hour. RPO target: 5 minutes.
- Runbook for full restore lives in `docs/backup-restore.md`.

## 18. On-Call Rotations

- Primary on-call carries pager 24/7 for one week.
- Secondary on-call available for Sev1 handoff.
- Escalation to lead engineering within 30 minutes of unresolved Sev1.
- Operations contact (branch coordination) always paired with primary.

## 19. Post-Incident Review

Every Sev1 and Sev2 produces:

- Timeline (UTC and branch-local for the most-affected branch).
- Blast radius (sessions, branches, organizations affected).
- RCA.
- Detection time, response time, resolution time.
- Action items with owners and dates.

Review meeting within 5 business days. Action items tracked in the incident tracker.

## 20. Communication Templates

For Sev1 customer-impact incidents, use the templates in `docs/incident-comms/`:

- Initial acknowledgement.
- Status update every 30 minutes.
- Resolution.
- Post-incident summary.

Templates omitted here intentionally; they live with operations.

## 21. Quick Reference

| Symptom | First action |
| --- | --- |
| Staff can't log in | Section 1 |
| Guests see "table occupied" everywhere | Section 8 |
| WS not updating | Section 3 |
| Payment stuck pending | Section 4 |
| Webhook 5xx storm | Section 5 |
| Flag rollback needed | Section 6 |
| Tenant data missing | Section 9 |
| Branch went offline | Section 13 |
