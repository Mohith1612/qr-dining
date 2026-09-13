# Findings ledger — current numbering (F-19 onward)

**Reconstructed 2026-09-12.** `findings-ledger-v2.md` and `remediation-plan.md` are not in this
repo and were not in the attachments — F-19 through F-29 existed only in session transcripts. This
file records what could be sourced from the working tree or established in session, and nothing
else. Where a finding is listed as fixed on someone's say-so rather than a citation, it says so.
If the real v2 ledger surfaces, merge into it and delete this.

> **ID collision — read before acting on any ID in this range.**
> `docs-before-rebuild:docs/history/operational-correctness-audit.md` (2026-05-21) uses its own F-numbering that
> overlaps this one. Its **F-27 is "Theme Management Is an Ad Hoc JSON Setting"** — unrelated to
> the F-27 below. An agent told to "close F-27" and left to locate it will find the wrong finding.
> Always cite the ledger alongside the ID.

| ID | Finding | Status |
|---|---|---|
| F-19 | Central authz role enforcement shadow-gated by default; seven routes reachable by the wrong role. Policy is correct, enforcement is off. | **Open.** In progress. |
| F-20 | Role-separation e2e specs broken at fixture level. | **Closed** — `seedOrg(label, {staffRole})` added; S-02, S-04, S-06, A-04, OPID-04 now get the identity their scenario claims. |
| F-21 | — | Fixed (no detail recovered). |
| F-24 | — | Fixed (no detail recovered). |
| F-25 | — | Fixed (no detail recovered). |
| F-26 | Cart endpoint leaks Go field names. | **Open.** In progress. |
| F-27 | `SESSION_CREATED` WS payload serialized `{Session, Participant}`, exposing `session_token` — a live guest credential — to every connected client and persisting it to `event_log` / `session_events`. | **Closed.** Code fixed. **No scrub and no credential rotation required:** R1 was local staging on the owner's machine (single instance, host-built binary, local PG/Redis, idle tail, no real traffic) and no real guest has ever scanned a QR on any deployed instance. Every leaked token is synthetic. Drop and reseed the test databases when convenient; **leave the soak DB alone — it is certification evidence.** |
| F-28 | Three further blanket-500 error mappings. | **Open.** |
| F-29 | Payment page loses "awaiting confirmation" on reload. | **Open.** |

## Cluster E

| Item | Status |
|---|---|
| Staff logout does not revoke server-side — the registered handler only clears the cookie, while the PIN-rotation and deactivation paths do perform revocation. (`audit/SYSTEM-AS-BUILT.md` §3.1 records this contradiction.) | **Open.** |
| Idempotency keys are never expired or reaped, in the payment path and the order path. | **Open — restate before assigning.** See below. |

### Correction to the idempotency finding

"Never expire" is not accurate, and the accurate version is worse. `expires_at` **is** written on
insert (`backend/internal/repository/idempotency.go:29`, 24h for payments), but
`backend/sql/queries/idempotency.sql` contains exactly four queries — `CreateIdempotencyKey`,
`GetIdempotencyKey`, `CompleteIdempotencyKey`, `FailIdempotencyKey` — and **not one of them
references `expires_at`**. No cleanup job references idempotency at all.

So there are two defects, not one:

1. **The recorded expiry is never enforced.** `GetIdempotencyKey` matches on scope and key alone.
   A replayed key returns the stored `response_resource_id` regardless of age — an arbitrarily old
   key still replays to its original payment instead of being treated as a fresh request.
2. **Nothing reaps the table.** Unbounded growth.

Fixing only (2) leaves the replay window infinite while making the table look maintained. Whoever
takes this should be told to fix (1) first, and to write the failing test as a replay of a key
whose `expires_at` is in the past.

## Test-suite items

| Item | Status |
|---|---|
| L-03 | **Quarantine, do not repair.** Its title claims reconnect during `awaiting_reactivation`; the fixture builds only an active session and the worker needs five minutes of durable absence to reach that state. Changing the expected status code produces a vacuous green. Tracked as `UNREACHABLE` in the census. |
| ~42 vacuous specs | **Open** — step 0 Task A, `release-certification/prompts/step0-a-e2e-quarantine.md`. |

## Invariant status verified this session

Against code, with citations, in `audit/Invariance.md`: H2, H3, H6, P1, P2, P3, P4, P5, P6, S2, T1
corrected from stale markers. **H4 is the only status line on that page not verified from code.**
**No open decisions remain on that page** as of 2026-09-12. H5 is decided at three minutes and
shipped; S1 is decided as last-activity and is **decided but not built** — it needs its own task
with a failing test, and it touches two queries (`ListExpiredSessions` and
`ListSessionsExpiringSoon`), which must change together.
