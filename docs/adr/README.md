# Architecture decision records

ADRs retain the context, choice, and rejected alternatives from the date of the
decision. Their reasoning is historical; use [../ARCHITECTURE.md](../ARCHITECTURE.md)
and [../INVARIANTS.md](../INVARIANTS.md) for current implementation behavior.

| ADR | Decision | Date | Current verification |
|---|---|---|---|
| [0001](0001-signoz-adoption.md) | Adopt self-hosted SigNoz for distributed tracing alongside Prometheus | 2026-08-03 | Tracing is implemented but defaults off; setup continues without it when initialization fails (`backend/internal/config/config.go:214-225`; `backend/cmd/server/main.go:39-44`). No retained evidence proves the checked-in SigNoz deployment has run. |
| [0002](0002-r3-policy-decisions.md) | Ratify central-authorization policy semantics | 2026-05-29 | Tenant-scope denials are unconditional; role denials depend on `AUTHZ_CENTRAL_POLICY_ENFORCE` (`backend/internal/handlers/authz.go:48-103`). Code defaults false; manual testing sets true (`backend/internal/config/config.go:253-263`; `scripts/manual-testing-up.sh:62-75`). |
| [0003](0003-tenant-suspension-enforcement.md) | Enforce organization/branch suspension at guest entry, not on live sessions | 2026-09-12 | QR resolve, session create, and join use the gate (`backend/internal/services/menu.go:98-112`; `backend/internal/services/session.go:99-115,567-596`). Staff authentication still rejects inactive organization or branch status (`backend/internal/services/staff.go:179-197`). |

Number a new record sequentially and preserve its reasoning after acceptance.
When a decision changes, add a dated superseding note or a new ADR rather than
rewriting the old rationale.
