# Domain documentation for agents

Before changing behavior, read the maintained description in
[`docs/ARCHITECTURE.md`](../ARCHITECTURE.md), then the relevant rules in
[`docs/INVARIANTS.md`](../INVARIANTS.md). Read any applicable record in
[`docs/adr/`](../adr/), preserving the distinction between an ADR's historical
reasoning and the current implementation.

Do not use `audit/SYSTEM-AS-BUILT.md` as current behavior. It predates the
migration-40 payment guard
(`backend/migrations/000040_one_non_terminal_payment_per_session.up.sql:1-29`),
the current all-manual initiation state
(`backend/internal/services/payment.go:759-773`), and other remediation work.

Use the exact domain vocabulary already present in code and the maintained docs.
When code, tests, and prose disagree, cite the conflicting source lines and use
`docs/INVARIANTS.md` as the documented arbiter. If a proposed change contradicts
an ADR, name the record explicitly and add a superseding decision instead of
silently rewriting its rationale.

There is no requirement to create a `CONTEXT.md` before ordinary work. Add domain
documentation only when a resolved concept or decision needs a durable home.
