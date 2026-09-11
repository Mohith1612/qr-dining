# Reference contracts

Specifications the implementation is required to satisfy. A difference between one of these documents and the code is a **bug**, not a documentation gap — decide which side is wrong, then fix that side.

These files are cited by `§` number from code comments and tests. **Do not rename them.**

| Document | Contract | Cited by |
|---|---|---|
| [session-lifecycle-state-machine.md](session-lifecycle-state-machine.md) | Session states, transitions, freeze and resurrection rules, the terminal snapshot read window | `internal/services/session.go` |
| [payment-finalization-invariants.md](payment-finalization-invariants.md) | Payment correctness: settlement, idempotency, webhooks, bill snapshots, `payment_pending` entry/exit | `internal/db/sqlc/sessions.sql.go`, `internal/db/sqlc/querier.go` |
| [realtime-reconciliation-invariants.md](realtime-reconciliation-invariants.md) | Hub, pub/sub, sequencing, reconnect, presence, slow-consumer eviction | `internal/websocket/`, `internal/events/` |
| [security-hardening-checklist.md](security-hardening-checklist.md) | §-numbered security gate list | `internal/middleware/security_headers.go` (§9) |
| [websocket-events.md](websocket-events.md) | WebSocket envelope and event catalogue — the client-facing contract | frontend WS client |
| [reconnect-guide.md](reconnect-guide.md) | Client reconnect: snapshot reconciliation, backoff, the diff algorithm | frontend WS client |

## Currency

The three invariant documents carry dated "Phase A/B implementation status" headers recording what was wired when. Those headers are accurate; read them together with the body, since a `[ ]` further down may already have been implemented by a later phase.

`security-hardening-checklist.md` is the clearest example: its Phase A and Phase B notes describe rate limiting, lockouts, MFA and credential revocation as **done**, while checkboxes in the body still show them open. Trust the phase notes and [../SECURITY.md](../SECURITY.md) over the checkboxes.

The narrative security model is [../SECURITY.md](../SECURITY.md); the architecture that these contracts constrain is [../ARCHITECTURE.md](../ARCHITECTURE.md).
