# Pilot Abort Criteria

**Scope:** the first restaurant pilot. One site, one developer on call, no escalation chain.
**Companions:** [RUNBOOKS.md](RUNBOOKS.md) (contain an incident) · [OPERATIONS.md](OPERATIONS.md)
(day-2) · [RECOVERY.md](RECOVERY.md) (data loss).

> **Why this is a separate file and not a section of RUNBOOKS.md.**
> RUNBOOKS.md is organised by symptom and answers "how do I fix this?". You read it while you
> still believe you can fix the thing. This document answers a different question — "should we
> still be doing this at all?" — for a reader who has already failed to fix it, at 20:00 on a
> Friday, alone. It also contains a page written for restaurant staff rather than for an
> engineer. Burying it in an engineering runbook would make it harder to find at exactly the
> moment it is needed. RUNBOOKS.md links here before its first incident step and from billing
> discrepancy handling.

---

## 0. The one thing to read first

**Every threshold below is a pre-commitment.** They are written now, calmly, because the person
who has to apply them will be under pressure from a restaurant owner who has staff on shift and
guests in the room, with nobody to escalate to and nobody to say "stop." A solo on-call
engineer's failure mode is not panicking — it is debugging for two hours while the restaurant
quietly falls apart.

**The discipline: if you cannot state the root cause within the criterion's decision window, you
act on the criterion. You do not keep debugging.** Not knowing the cause *is* the answer.

And the constraint that shapes everything here:

> **Schema rollback is not available. The pilot is forward-fix only.**
> Down-migrations are unsafe across 16–24 and at 36, 38 and 39. Migration 22's up cannot be
> re-applied to a populated `audit_log` at all — it aborts on the immutability trigger and leaves
> `schema_migrations` dirty, which prevents the app from starting. Verified empirically on
> 2026-09-12: [RECOVERY.md, Track F verification record](RECOVERY.md#track-f-verification-record).
> "Roll back the schema" is not a response you have. §4 is what you have instead.

---

## 1. Abort criteria

Three outcomes, and it matters which one you are choosing:

| Outcome | Meaning |
|---|---|
| **FALLBACK** | This service runs on paper. The pilot continues tomorrow. Reversible, low-cost, use it early and often. |
| **SUSPEND** | No further services until a named fix ships and is verified. The pilot is paused, not dead. |
| **ABORT** | The pilot ends. Remove the QR collateral, reconcile, write the post-incident report. |

### A1 — A guest is charged incorrectly

| | |
|---|---|
| **Threshold** | **One** confirmed instance → FALLBACK for the rest of the service, root-cause within 24h. **A second independent instance**, or a first instance not root-caused within 24h → **ABORT**. |
| **Decision window** | Immediate for the fallback. 24h for the abort decision. |
| **Signal** | `billing_reconciliation_discrepancies{comparison}` (page at >0) plus one `billing.reconciliation.discrepancy` audit row naming the session and exact amounts. **This detects the system disagreeing with itself, not a guest being charged the wrong amount** — see §6, no-telemetry row 0. |

*Why this threshold.* The pilot is cash/UPI-manual: staff read the total off the screen and
collect money in the room. A wrong total is immediate, real financial harm to a guest or to the
restaurant, and there is no refund path in the product — the billing subsystem is shadow and does
not charge or refund anyone. One instance can be a staff misread or a guest misremembering, and
the immutable `bill_snapshots` row lets you tell which. Two independent instances cannot be
coincidence, and a system that gets money wrong is the one thing a restaurant will not forgive.

*Definition of "confirmed":* the `bill_snapshots` row for that session disagrees with the order
lines, or with what the guest was asked to pay. Confirm from the snapshot, not from memory.

The reconciliation worker runs every five minutes over sessions closed in the preceding 24 hours,
after a one-minute quiet period. It compares exact `NUMERIC(12,2)` amounts: snapshot versus the
snapshot's bill-time order lines, and all completed collections versus the authoritative snapshot.
Cancelled payments and staff force-closes are excluded. The second-instance and 24-hour root-cause
conditions remain manual because neither independence nor investigation status exists as a metric.
This signal observes what staff marked completed in the app; it cannot detect a different amount of
cash/UPI actually handed over if the app record itself says the expected amount.

### A2 — Any cross-tenant data exposure

| | |
|---|---|
| **Threshold** | **One confirmed instance → ABORT immediately.** No threshold, no grace period. |
| **Decision window** | Immediate. |
| **Signal** | `CrossTenantScopeDenial` (page) for blocked attempts. **Successful exposure has no signal.** |

*Why no threshold.* A single-site pilot means the only other tenants on the box are test
organizations, so the likelihood is low — but the 2026-08-04 independent audit found a real
cross-org write bypass in this codebase, which means the class of defect is proven present, not
theoretical. The cost is unbounded: it is a disclosure obligation, it is the one failure that
makes the product unsellable to the next restaurant, and unlike a wrong bill it cannot be made
right afterwards. There is no amount of cross-tenant exposure that is acceptable in a pilot whose
entire purpose is to earn a reference customer.

*Note on enforcement:* tenant scope denials are enforced unconditionally, independent of
`AUTHZ_CENTRAL_POLICY_ENFORCE` (`backend/internal/handlers/authz.go:48-72`). R3 being false does **not** mean scope
is unguarded. Role-policy denials are still shadow-only while R3 is off.

### A3 — A table cannot order or pay

| | |
|---|---|
| **Threshold** | **One table** blocked > **15 minutes** during service, **or ≥2 tables simultaneously** blocked > **5 minutes** → FALLBACK for the service. Recurrence on **≥2 separate services** → **ABORT**. |
| **Decision window** | 15 minutes from the staff report. Do not exceed it diagnosing. |
| **Signal** | **None per-table.** Systemic failures show as 5xx on `/sessions/:id/orders` and `/sessions/:id/payments`; a single wedged table does not. See §5. |

*Why 15 and 5.* Fifteen minutes is about one course cycle — past that the guest has noticed, and
a QR system that makes guests wait is worse for the restaurant than no QR system. Five minutes
for two tables because simultaneity means it is the software rather than a device or a torn QR
code, and a systemic fault will reach every table in the room shortly. Two separate services
means it is not an unlucky night.

*"Blocked" means:* the guest cannot place an order or cannot reach a bill, and re-scanning the QR
does not fix it. A single guest's dead phone battery is not this criterion.

### A4 — Data loss

| | |
|---|---|
| **Threshold** | **Any confirmed loss of committed order, payment or audit data → ABORT immediately.** Separately: **no verified backup for 48h** → SUSPEND; not fixed within a further 24h → **ABORT**. |
| **Decision window** | Immediate for confirmed loss. |
| **Signal** | `BackupFailed`, `BackupTooOld` (36h), `BackupStaleEarlyWarning` (26h), `AuditWriteFailureAny`. **All are partially untrustworthy — see §5.** |

*Why 48h for backups.* RPO is ≤24h by design and `BackupTooOld` pages at 36h. Reaching 48h means
a page fired and was not acted on. At that point you are operating a money system with no
recovery path, which is a different risk from the one the pilot was scoped to accept.

*Why immediate for confirmed loss.* Every other criterion is about degraded service. This one is
about the record of what happened being gone. The restaurant's accounts depend on it.

### A5 — Sustained backend unavailability during service hours

| | |
|---|---|
| **Threshold** | **>10 consecutive minutes** unavailable during service, **or >30 cumulative minutes** in one service → FALLBACK for that service. **Third occurrence** across the pilot → **ABORT**. |
| **Decision window** | 10 minutes. |
| **Signal** | `AppTargetDown`, `ReadyzProbeFailing` (both page at 1m), `AppUnavailableAbortThreshold` (10m). Good coverage. |

*Why 10 minutes.* Readiness checks both PostgreSQL and Redis independently
([RUNBOOKS.md, First minute](RUNBOOKS.md#first-minute)). Ten minutes is enough for one restart
and one diagnosis attempt by one person. If a restart did not fix it in ten minutes, the next
step is a build or a restore, neither of which belongs in the middle of a service.

*Why 30 cumulative.* A service that flaps for half an hour has already been run on paper by the
staff; calling it a fallback just makes it official and gets you honest data about what happened.

*Why the third occurrence.* Once is an incident. Twice is bad luck. Three times is the system's
actual reliability, and it is not good enough to put in front of paying guests.

### A6 — Schema wedge (`schema_migrations.dirty = true`)

| | |
|---|---|
| **Threshold** | Not resolved within **60 minutes** → **ABORT**. |
| **Decision window** | 60 minutes, and the app is down for all of it. |
| **Signal** | **No direct signal.** `AppTargetDown` fires because the app will not start; nothing names the cause. See §5. |

*Why this is its own criterion.* A dirty migration is not a normal outage. The app refuses to
start, and — if the wedge is at 22 — recovery requires **dropping the `audit_log` immutability
trigger by hand on a live database** ([RUNBOOKS.md, Database failure](RUNBOOKS.md#database-outage-corruption-or-migration-failure)). That
trades the tamper-evidence guarantee for uptime. `migrate force <version>` and `migrate version`
exist for this (added 2026-09-12) and the procedure is regression-tested, but it is still a
procedure you should perform at most once. Sixty minutes is roughly what it takes when done
carefully by someone who has read it before.

*Prevention beats recovery here:* see §7, "No deploys during service hours."

### A7 — Audit integrity loss

| | |
|---|---|
| **Threshold** | Any `audit_write_failures_total` increase, or the `trg_audit_log_immutable` trigger found absent → FALLBACK immediately; not restored within the same service → **ABORT**. |
| **Decision window** | Immediate. |
| **Signal** | `AuditWriteFailureAny` (page), `AuditWriteFailures` (page). **Trigger absence has no signal.** |

*Why.* R1 (`AUDIT_LOG_V2_ENABLED`) is the one flag marked never-disable, and `audit_log` is the
only tamper-evident record of who did what to money. A gap cannot be backfilled after the fact —
the rows that were not written do not exist. If the §4 step-6 procedure was used, this criterion
is the reason you re-create the trigger before the restaurant opens, not afterwards.

---

## 2. What is *not* an abort criterion

These look alarming and are contained by design. Treating them as aborts would end a pilot that
is working.

| Symptom | Why it is fine | Runbook |
|---|---|---|
| Redis down | Readiness fails; realtime, authentication caches, presence, sensitive rate limits, and worker locks depend on Redis. Durable business data remains in PostgreSQL. | [Redis outage](RUNBOOKS.md#redis-outage) |
| Payments stuck in `payment_pending` | Alert-only **by design** — the system never auto-settles. A human settles or cancels through the staff UI. The frozen cart is the invariant working. | [Stuck payment](RUNBOOKS.md#stuck-payment) |
| Webhook replay storm | The receipt insert suppresses a repeated external event ID after signature verification. | [Payment webhook rejection or replay](RUNBOOKS.md#payment-webhook-rejection-or-replay) |
| "Table occupied but empty" | The stale-session cleaner and reconciler have repair paths; staff can force-close. | [Stale session or occupied table](RUNBOOKS.md#stale-session-or-occupied-table) |
| `policy_shadow_mismatch_total` non-zero | This is a shadow signal only in an environment where central role enforcement is off; the application default is false, while the manual stack enables it. | [SECURITY.md, environment-dependent enforcement](SECURITY.md#authorization-and-environment-dependent-enforcement) |
| One guest's device misbehaving | Re-scan. Not A3 unless the table stays blocked past 15 minutes. |

---

## 3. Who decides, and how fast

**The on-call developer decides. There is no escalation chain, and inventing one during an
incident wastes the window.**

The restaurant's manager on duty holds one power and it is not the abort: **they may call
FALLBACK at any time, for any reason, without consulting anyone.** They own the service and they
can see the room. Make sure they know this before day one — a manager who feels they need
permission to go back to paper will instead let the room degrade while waiting for a call back.

**Nobody may call ABORT except the on-call developer**, and it is a decision made *after* the
service, never during one. The in-service decision is always FALLBACK.

### The clock

| Elapsed | What must have happened |
|---|---|
| **T+0** | Criterion observed. Say out loud (or write down) which criterion and what its window is. |
| **T+5** | One containment action attempted — restart, flag flip, or the runbook's stated containment. |
| **T+10** | If unresolved: **tell the restaurant to fall back to paper.** Do not wait until you are sure. Falling back early and being wrong costs one service of convenience; falling back late costs the room. |
| **T+30** | Cumulative debugging cap for one service. Stop. The service is on paper, you are no longer under time pressure, and you diagnose properly afterwards. |
| **Next morning** | ABORT/SUSPEND/CONTINUE decision, in writing, against the criteria above. |

**Write the decision down at T+0.** A criterion silently reinterpreted at T+45 is not a criterion.

---

## 4. What "abort" actually does

### The case to plan for

**20:00 Friday. Four tables mid-meal. Two have ordered and not paid, one is waiting on food, one
has just scanned.** A clean shutdown is not available: those four tables must still be able to
eat and pay, and the restaurant must still be able to close its till.

**Abort is not `docker compose down`. It is: stop the inflow, drain what is in flight, then
stop.**

### Step 1 — Stop new sessions (instant, no deploy, physical)

**Staff collect the QR table tents.** That is the primary lever and it is the right one: it takes
30 seconds, it needs no deploy, no flag, and no working backend, and it cannot affect the four
sessions already in flight. Every software mechanism for blocking new sessions is slower, riskier
and can hurt the tables you are trying to protect.

Do **not** stop the app to stop new sessions. That breaks the four tables mid-meal.

### Step 2 — Drain the four tables

**If the backend is healthy** (the abort is for A1, A2 or a post-service decision): let them
finish normally. Guests order and pay as usual; staff settle as usual. Nothing special happens.
This is the common case and it is calm.

**If the backend is unhealthy** (A5, A6), the staff UI is the fallback, and if that is gone, paper:

1. **Get the bill totals out.** Use the staff surface while the API is reachable. If the API is
   down but Postgres is readable, use the recorded break-glass database read
   ([RUNBOOKS.md, Break-glass database read](RUNBOOKS.md#break-glass-database-read)). A
   `bill_snapshots` row exists only after payment initiation; otherwise calculate from the placed
   order-item price snapshots. Record the read before doing it
   (`backend/migrations/000001_initial_schema.up.sql:218-250`,
   `backend/migrations/000021_payment_order_correctness.up.sql:48-64`).
2. **Settle on paper.** The restaurant takes cash or UPI against the written total and keeps the
   slip. This is already how money moves in the pilot — the system records the settlement, it
   never moves the money.
3. **Close the sessions out** once the app is back, so the tables free up:
   `PATCH /payments/:id/cancel` and `POST /sessions/:id/force-close`, both available in the staff
   UI, both requiring a reason, both audited. Without these a non-terminal payment freezes its
   session and the table wedges.

> **Known gap, tell the kitchen in advance.** The kitchen display is the only record of in-flight
> orders. If the backend dies mid-service, orders already placed but not yet cooked are not
> visible anywhere the kitchen can reach. **Pre-pilot requirement: the kitchen keeps a written
> ticket or a printed docket for every order.** This costs nothing while things work and is the
> only thing between you and a lost course when they do not.

### Step 3 — Only after the last table has paid

```bash
cd /opt/qr-dining
# 1. Final backup FIRST. This is evidence — for the post-incident report and for
#    the restaurant's accounts. Do it before anything else.
docker run --rm --network qr-dining_internal --env-file /opt/qr-dining/backup.env \
  -v /opt/qr-dining/repo/backend/scripts:/scripts:ro \
  postgres:17-alpine sh -c 'apk add --no-cache bash aws-cli >/dev/null && bash /scripts/nightly-backup.sh'
# 2. Verify it landed and its sha256 matches the manifest (RECOVERY.md, Backup artifact and selection).
# 3. Then, and only then:
docker compose stop app
```

**Never `docker compose down -v`.** Never delete the volume. The database is the evidence.
Keep the final dump for at least as long as the restaurant's accounting period.

### Step 4 — Reconcile, same night if possible

Export the day's sessions, orders and payments and reconcile against the till. Any discrepancy is
an A1 finding regardless of what triggered the abort. Then write the post-incident report
([RUNBOOKS.md, After containment](RUNBOOKS.md#after-containment)).

---

## 5. Recovery paths — forward-fix only

**Roll back the schema** is absent from this list on purpose. It is not available. See §0.

Try in order. Stop at the first one that works.

| # | Action | When | Cost | Safe? |
|---|---|---|---|---|
| 1 | `docker compose restart app` | Worker panic, pool exhaustion, wedged goroutine | ~12 s | Yes. Always try first. |
| 2 | **Flag flip** — edit `.env`, `docker compose up -d app` | An enforcement flag is denying legitimate traffic | <5 min | Yes, **except R1**. `AUDIT_LOG_V2_ENABLED` is never rolled back. R4/R5/R6 reversals are Sev1 decisions requiring explicit approval and are time-bounded, never a resting state. |
| 3 | **Revert the binary, not the schema** — pin the previous `IMAGE_TAG`, `docker compose up -d app` | A bad deploy, **when that deploy added no migration** | ~1 min | Yes *if and only if* the release spans no migration. **Confirm this before the pilot, not during an incident:** know which tag you are rolling back to and whether it crosses one. |
| 4 | **Freeze writes** — `docker compose stop app` | You need the database to stop changing while you think | Instant | Yes, but the restaurant is on paper from this second. Guests see 502s. |
| 5 | **Hotfix forward** | Nothing above applies | **≥1 hour** — one developer, arm64 build, no CI-gated path to a running container in minutes | Not a during-service option. During service the answer is paper. |
| 6 | **Restore the database** ([RECOVERY.md, Production restore](RECOVERY.md#production-restore)) | Data corruption or bad mutation only | ~5 min mechanical; loses ≤24h | Only **into the same schema version the dump was taken at**. A dump older than the running binary's schema **cannot be replayed forward** — see §0. |
| 7 | **Unwedge a dirty migration** — `migrate version`, `migrate force <v>`, then [RECOVERY.md, version-22 wedge](RECOVERY.md#known-dirty-migration-wedge-at-version-22) | A6 only | ~60 min | **Last resort.** Requires dropping `trg_audit_log_immutable`, replaying, and re-creating it. While the trigger is off, `audit_log` is writable and the tamper-evidence guarantee does not hold. Record that window in the incident report. |
| 8 | **Stop the pilot** | §1 says so | — | §4. |

### The rule that removes most of this

**No deploys during service hours.** Deploy before opening, with a verified backup taken first,
and watch `/readyz` and the gate metrics through one synthetic guest journey before the doors
open. Every path from #5 down exists because a deploy went wrong at the worst time. A deploy
window costs one morning; a mid-service migration failure costs a service and possibly the pilot.

---

## 6. Telemetry per criterion

**Delivery, as of 2026-09-16: alerts reach a phone.** Alertmanager routes
`severity: page` to a Telegram receiver that notifies and `severity: ticket` to
one that stays silent, both with `send_resolved: true`, and a permanently-firing
`DeadMansSwitch` pings an external watchdog every 2m30s so that the pipeline's
own death is not mistaken for quiet
([OPERATIONS.md, Alert delivery](OPERATIONS.md#alert-delivery)). Every row in the
first table below now ends at a human, and a pipeline that stops carrying them
reaches the operator by email — a path with nothing on this VM in it — in about
12 minutes, 14 worst case.

**What that did not change: the second table.** Delivery is the last hop. A
criterion with no signal has nothing to deliver, and building a channel does not
build a detector. Read both tables before assuming coverage.

### Criteria with a working signal

| Criterion | Signal | Alert | Severity |
|---|---|---|---|
| A1 guest charged incorrectly | `billing_reconciliation_discrepancies{comparison}` | `BillingReconciliationDiscrepancy` | page immediately at one finding; second-instance / 24h root-cause decision remains manual |
| A2 cross-tenant (blocked attempts) | `authz_denied_total{reason=~"branch_mismatch\|org_mismatch"}` | `CrossTenantScopeDenial` | page, `increase() > 0` |
| A4 backup stale | `qr_dining_backup_last_success_timestamp_seconds` | `BackupStaleEarlyWarning` (26h), `BackupTooOld` (36h) | ticket / page |
| A4 backup failed | `qr_dining_backup_last_run_status` | `BackupFailed` | page — **but see gaps** |
| A5 app down | `up{job="qr-dining"}`, `probe_success{job="qr-dining-readyz"}` | `AppTargetDown`, `ReadyzProbeFailing` (1m), `AppUnavailableAbortThreshold` (10m) | page |
| A5 datastore down | `/readyz` 503, body names which | `ReadyzProbeFailing` | page |
| A7 audit writes failing | `audit_write_failures_total` | `AuditWriteFailureAny`, `AuditWriteFailures` | page |

### Criteria with NO telemetry

**An abort criterion nobody can observe is decoration.** These are decoration until someone
builds the signal. They are listed so that the detection plan is explicit rather than assumed.

| # | Criterion | Why no signal | How it is actually detected | To fix |
|---|---|---|---|---|
| 0 | **A1 — a guest is charged incorrectly (the half that reaches the guest)** | `BillingReconciliationDiscrepancy` compares the system against *itself*: snapshot vs. its bill-time order lines, collections vs. snapshot. If the app recorded ₹840 and the guest handed over ₹1,840, every comparison agrees and the gauge stays zero. The pilot is cash/UPI-manual, so the money never passes through anything the app can observe. | **The guest notices**, or the till does not reconcile at close. Neither is real time. | Nothing cheap, and nothing in software. The compensating control is §8's instruction to staff — *if a total looks wrong, stop and call before taking the money* — and the same-night till reconciliation in §4 step 4. |
| 1 | **A3 — one table cannot order or pay** | App metrics are aggregate, none per-table. `active_sessions_total` is a count. 5xx on `/sessions/:id/orders` catches a systemic break, never one wedged table. | **Staff tell you**, by phone. The 15-minute window in A3 starts when they call, not when it broke. | A per-branch gauge of sessions with no state transition in N minutes. Needs code. |
| 2 | **A2 — *successful* cross-tenant read** | `authz_denied_total` counts what policy **blocked**. A read the policy wrongly **allows** emits nothing. The 2026-08-04 bypass was exactly this shape. | Audit-log review after the fact, or a report. Not in real time. | Nothing cheap. Accept and compensate with A2's zero-tolerance threshold. |
| 3 | **A6 — dirty migration** | `schema_migrations.dirty` is a table column, not a metric. `AppTargetDown` fires because the app will not start; nothing names the cause. | Read the app logs after the app fails to start, or run `migrate version`. | Export `schema_migrations.dirty` as a gauge at startup, or a node_exporter textfile check. Needs code or a cron. |
| 4 | **A7 — immutability trigger absent** | No metric. Nothing notices `trg_audit_log_immutable` missing — including after the §5 step-7 procedure re-creates it *or fails to*. | The manual check in §7. | A daily check that `UPDATE audit_log` still fails, exported as a gauge. Scriptable without code. |

### Gaps in the signals that *do* exist

- **`BackupFailed` covers failures after the script starts** — `nightly-backup.sh` installs an
  `EXIT` trap before validating `DATABASE_URL`, so its own validation and command failures write
  failure status (`backend/scripts/nightly-backup.sh:27-85`). **A scheduler or wrapper failure
  that never starts the script still writes nothing** (`deploy/backup/crontab.example:6-18`).
  That gap is covered only by `BackupStaleEarlyWarning` (26h) and `BackupTooOld` (36h), provided
  Prometheus receives the textfile series — so read the success timestamp directly, per §7.
- **Backup metrics only reach Prometheus if `NODE_EXPORTER_TEXTFILE_DIR` points at the
  `qr-dining_backup_textfile` volume mountpoint.** If that wiring is missing, all four backup
  alerts are silently dead ([OPERATIONS.md, Backup and restore](OPERATIONS.md#backup-and-restore)).
- **Delivery is one Telegram chat on one phone, and that phone is a single point of
  failure.** A muted chat, a flat battery, or no signal in the dining room and every page in
  this document is again a page to nobody — with the one exception of the dead man's switch,
  which is delivered by an external service and therefore survives this VM. The compensating
  control is unchanged and non-technical: the restaurant can call FALLBACK without reaching
  anyone (§3).
- **A silence is indistinguishable from a working pipeline.** `amtool silence add` during a
  planned deploy is correct practice; an unexpired silence the next evening is silent failure.
  `amtool silence query` is in the daily check in §7 for that reason
  ([RUNBOOKS.md, Silencing alerts for planned work](RUNBOOKS.md#silencing-alerts-for-planned-work)).

---

## 7. Before day one

Nothing in this document works if these are not true. Check them, do not assume them.

- [x] **Alertmanager has a real receiver and one test alert has been delivered end to end.**
      Done 2026-09-16: Telegram receivers for page/ticket, verified by a hand-injected alert,
      by a real `AppTargetDown` firing and resolving, and by confirming inhibition suppressed
      its dependants ([OPERATIONS.md, Verifying delivery](OPERATIONS.md#verifying-delivery-end-to-end)).
- [x] **The dead man's switch is wired and its failure mode has been tested.** Not that the
      ping arrives — that proves nothing — but that **stopping Prometheus makes the external
      watchdog report the check late**. Verified 2026-09-16: Prometheus stopped `10:53:52Z`,
      last ping `10:56:04Z`, healthchecks.io `status: "grace"` at `11:02:04Z`, down flip at
      `11:06:04Z`, recovery on the first ping after restart. Both the down and the recovery
      email arrived. **Re-test this whenever the observability stack is touched** — it is the
      only check in this document that cannot be verified by watching it succeed.
- [ ] **`/opt/qr-dining/repo` is on `main` and not diverged** (`git status -sb | head -1`).
      The observability containers bind-mount their config straight out of that checkout, so
      its branch *is* the running alert configuration
      ([OPERATIONS.md, The new hazard](OPERATIONS.md#the-new-hazard-the-checkout-is-now-production-state)).
- [ ] **The rule count Prometheus actually loaded matches the repo.** On 2026-09-16 the box
      was evaluating 27 of 32 rules and the five pilot-abort alerts were simply absent — a
      rule that was never loaded looks exactly like a rule that is not firing. `curl -s
      localhost:9090/api/v1/rules` and count
      ([OPERATIONS.md, Keeping the VM in step with the repo](OPERATIONS.md#keeping-the-vm-in-step-with-the-repo)).
- [ ] **The operator's phone can actually receive the page**: the Telegram chat is unmuted,
      notifications survive the phone's Do Not Disturb / focus mode at 20:00, and the watchdog's
      own notification channel (healthchecks.io / UptimeRobot email or app) is on a *different*
      path than Telegram. Check this on the handset that will be in the room, not on a desktop.
- [ ] `NODE_EXPORTER_TEXTFILE_DIR` verified to be the `qr-dining_backup_textfile` mountpoint, and
      `qr_dining_backup_last_success_timestamp_seconds` confirmed visible in Prometheus.
- [ ] One restore drill at real data volume, timed
      ([RECOVERY.md, Rehearsed scratch restore](RECOVERY.md#rehearsed-scratch-restore)).
- [ ] `backend/scripts/tests/backup-restore-test.sh` passes on the deployment host — it is the
      cheapest proof that the backup and restore path still behaves as this document assumes.
- [ ] **The rollback target tag is known**, and whether it spans a migration is known.
- [ ] The restaurant manager knows they can call FALLBACK unilaterally, and how.
- [ ] The kitchen has a paper ticket process and has used it once in a dry run.
- [ ] Staff have practised: force-close a session, cancel a payment, read a bill total from the
      staff UI.
- [ ] QR table tents are removable in under a minute, and someone on each shift knows that is the
      abort lever.
- [ ] This document has been read once, calmly, by the person who will be on call.

**Daily during the pilot** (~2 minutes, before service):

- [ ] `qr_dining_backup_last_success_timestamp_seconds` is under 26h old — **read the value, do
      not trust the absence of an alert**.
- [ ] `UPDATE audit_log SET action='x' WHERE id=(SELECT min(id) FROM audit_log);` fails. If it
      succeeds, that is A7.
- [ ] `SELECT version, dirty FROM schema_migrations;` reads `40, false`.
- [ ] No firing alerts **other than `DeadMansSwitch`**, which fires permanently by design;
      all Prometheus targets UP.

      > As of 2026-09-16 the soak deployment also has a standing
      > `PaymentPendingEscalationCritical` for a genuinely wedged session — a non-terminal
      > payment holding its table. **That is not noise and must not be silenced.** It is
      > precisely what the alert is for, and it is the alert an earlier, over-broad inhibit
      > rule was found to be hiding. Clear it by settling or cancelling the payment through
      > the staff UI ([RUNBOOKS.md, Stuck payment](RUNBOOKS.md#stuck-payment)); the alert
      > resolves itself once the session can close.
- [ ] No leftover silences: `docker exec qr-dining-alertmanager-1 amtool
      --alertmanager.url=http://localhost:9093 silence query` is empty. A silence from
      yesterday's deploy and a working alert pipeline look identical from the outside.
- [ ] The watchdog check reads "up" in healthchecks.io / UptimeRobot. If it is late, alerting
      is down and nothing else on this list is trustworthy.

---

## 8. What the restaurant is told

*Give the manager this page. Do not give them the rest of this document.*

### Before the pilot

> The QR ordering system is new and we are testing it with you. **Paper is always available and
> using it is never a failure.** If anything about the system is making service worse — slow,
> confusing, wrong totals, anything — **tell your staff to take the QR cards off the tables and
> serve the rest of the night on paper.** You do not need to ask us first. Call us afterwards,
> or during if you can spare someone.
>
> Three things we need from you while we test:
>
> 1. **The kitchen writes down every order**, even though it appears on the screen. If the system
>    goes down mid-service, the paper ticket is how the food still gets made.
> 2. **If a bill total looks wrong, stop and call us before taking the money.** Do not correct it
>    yourself. Tell us the table and the time; we can see exactly what the system calculated.
> 3. **Tell us when something is annoying**, not just when it is broken. Annoying is what we are
>    here to find.
>
> Our number is `<ON-CALL NUMBER>`. One person is on call. If you get voicemail, go to paper and
> we will call back.

### If a table is mid-meal and we stop the system

> Nothing is lost and nobody has to leave.
>
> - **Guests already eating finish normally.** Take the order and the payment on paper.
> - **The bill total is on the staff tablet** under the table's session. Write it down and take
>   payment as you normally would for a cash table. Keep the slip.
> - **If the tablet is not working either**, add up the order from the kitchen tickets. Charge
>   the guest what they ordered — if you are unsure, charge less and tell us. We will reconcile
>   with you afterwards.
> - **Take the QR cards off every table** so nobody new starts an order.
> - **Do not worry about closing the tables in the system.** We will do it. If a table shows as
>   occupied tomorrow morning, that is our problem, not yours.
>
> The guests in the room come first. The data can be fixed later; a ruined dinner cannot.

### If we abort the pilot

> We are stopping the trial. This is our decision and it reflects our software, not your staff.
>
> - We will take the QR cards and the table stands away.
> - We will give you a written summary of what went wrong and anything that affected your money
>   or your guests.
> - Your data stays yours. If you want an export of everything that went through the system, say
>   so and we will provide it.
> - If any guest was charged incorrectly during the trial, we will tell you which table and when,
>   so you can make it right.
