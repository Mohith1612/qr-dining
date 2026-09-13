# Reference

Last verified: 2026-09-13.

- [Payment finalization](payment-finalization-invariants.md) is the narrow money
  concurrency contract; the implementation is in the payment handler/service and
  migration 40 (`backend/internal/handlers/payment.go:105-193`,
  `backend/internal/services/payment.go:128-355`,
  `backend/migrations/000040_one_non_terminal_payment_per_session.up.sql:1-29`).
- [Realtime reconciliation](realtime-reconciliation-invariants.md) defines durable
  sequence, replay, and snapshot replacement behavior
  (`backend/internal/repository/session.go:200-281`,
  `backend/internal/services/session.go:651-779`).
- [Session lifecycle](session-lifecycle-state-machine.md) is the detailed state
  reference generated from the domain transition table
  (`backend/internal/domain/statemachine.go:19-47,82-107`).
- [WebSocket events](websocket-events.md) lists only current producers and marks
  declared-but-unused events (`backend/internal/websocket/message.go:13-61`,
  `backend/internal/events/events.go:86-183`).
- [Security hardening checklist](security-hardening-checklist.md) is a review aid;
  the narrative security model is [../SECURITY.md](../SECURITY.md).
