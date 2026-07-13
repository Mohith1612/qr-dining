# Recovery & Emergency Procedures

Last-resort procedures for pilot operations. Prefer the governed surfaces (Support Console,
billing, platform lifecycle) over anything here. **Test every destructive step against an
isolated DB first; never improvise against production.**

> Discipline: do not reset the soak/production DB, do not mutate `audit_log`, and preserve
> the strict-rollout flags across restarts (see `master-system-context-v1.md` §14.6).

## A. Database restore (data loss / corruption)
Scripts: `backend/scripts/{backup,restore,backup_r2}.sh`; runbook: `docs/backup-restore.md`.
1. Stop writes if possible (take the app out of rotation).
2. Identify the target backup (pg_dump custom format; R2-uploaded with retention pruning).
3. Restore single-transaction into a **fresh** database, verify, then cut over.
4. Post-restore: confirm `/readyz` (DB+Redis), audit immutability trigger intact, migration
   version matches, and spot-check a recent session/payment in the Support Console.

## B. Rollout flag rollback (a flipped enforcement flag misbehaves)
The 9 strict flags are **reversible**: set the flag `false` and restart (MTTR < 5 min, no data
risk). Confirm which flag via `internal/config/config.go` + the metric/alert that fired
(`audit_write_failures_total`, `policy_shadow_mismatch_total`, `tenant_resolution_failures_total`,
etc.). Roll **one** flag back at a time; re-check the dashboards/alerts; document the rollback.
> Never chain rollbacks blindly — paired flags (AUTHZ+STRICT branch; STAFF_CODE+STAFF_SESSION_DB)
> flip/roll back **together**.

## C. Tenant disable (abuse / non-payment / emergency)
This is **governance, not deletion** — fully reversible, fully audited.
1. **Org:** `POST /platform/organizations/:id/suspend` (or the Suspend button on org detail).
2. **Branch:** `POST /platform/branches/:id/suspend`.
3. **Subscription:** suspend on the billing page (records + audits the state).
> **Caveat (current phase):** these status flips are **inert** — no operational path reads them
> yet (resolve→observe→enforce). To truly cut off a tenant **today**, you must also revoke staff
> access (deactivate staff in Admin) and/or rotate table QR tokens. Track real enforcement in the
> Observability surface; full enforcement is a deliberate future rollout behind new flags.
4. Re-enable: the matching `…/activate` endpoints + reactivate the subscription.

## D. Emergency support (active incident)
1. **Triage signal:** `/readyz` is the authoritative outage signal (not the pub/sub gauge).
2. **Scope it:** one tenant vs platform-wide? Use Support Console + Observability + `/metrics`.
3. **Comms:** note start time; if guest-facing, expect graceful degradation (clients reconnect +
   snapshot-reconcile; no business data lost on Redis loss).
4. **Stabilize, don't fabricate:** never auto-settle/auto-cancel payments or hand-edit `audit_log`.
   Bring the app back (it's stateless; Postgres is the source of truth), then reconcile via the
   console.
5. **Deployment note:** do not run the app from a path that a cleanup can wipe (the soak learned
   this the hard way, CP-4) — ensure a liveness/app-down alert exists before a production soak.

## E. Post-incident
- [ ] Write a short timeline (what, when, signal, action, MTTR).
- [ ] Confirm audit trail captured the operator actions (`platform_audit_log`).
- [ ] File follow-ups (e.g. enforcement gaps surfaced, missing alerts).
