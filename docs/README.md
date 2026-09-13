# Documentation index

This index is the entry point for maintained documentation. System behavior is
described only where the referenced source lines support it; historical material
removed from the working tree remains retrievable through the
`docs-before-rebuild` tag.

| Document | Reader and purpose | Last verified |
|---|---|---|
| [ARCHITECTURE.md](ARCHITECTURE.md) | Engineers: durable model, HTTP identities, state machines, money path, realtime, and workers | 2026-09-13 |
| [INVARIANTS.md](INVARIANTS.md) | Reviewers and test authors: the code-derived arbiter for test-versus-code disputes | 2026-09-13 |
| [DEVELOPMENT.md](DEVELOPMENT.md) | Contributors: setup, exact checks, environment flags, and manual stack | 2026-09-13 |
| [TESTING.md](TESTING.md) | Contributors and reviewers: suite boundaries, integration tags, migration check limits, and the Playwright vacuity census | 2026-09-13 |
| [OPERATIONS.md](OPERATIONS.md) | Operators: deploy configuration, backup wiring, metrics, alerts, and worker behavior; unexecuted procedures are marked unverified | 2026-09-13 |
| [RUNBOOKS.md](RUNBOOKS.md) | On-call engineer: symptom-oriented containment and evidence gathering | 2026-09-13 |
| [RECOVERY.md](RECOVERY.md) | Operator handling data loss or migration failure: rehearsed backup/restore and dirty-migration recovery | 2026-09-13 |
| [SECURITY.md](SECURITY.md) | Security reviewers and engineers: identity classes, credential lifecycle, authorization, projections, and known gaps | 2026-09-13 |
| [PILOT-ABORT-CRITERIA.md](PILOT-ABORT-CRITERIA.md) | Pilot owner and restaurant staff: precommitted fallback, suspension, and abort thresholds | 2026-09-12 |
| [adr/](adr/) | Engineers: accepted architectural decisions and their reasoning | See each ADR |
| [reference/](reference/) | Implementers: smaller code-derived state, payment, realtime, and security references | 2026-09-13 |
| [manual-testing/testing-dashboard.html](manual-testing/testing-dashboard.html) | Human tester: launcher, procedure, current surfaces, data map, and local observation checklist | 2026-09-13 |
| [agents/domain.md](agents/domain.md) | Coding agents: order and authority of implementation, invariant, and ADR documents | 2026-09-13 |
| [agents/issue-tracker.md](agents/issue-tracker.md) | Coding agents: GitHub issue command conventions | 2026-09-13 |
| [agents/triage-labels.md](agents/triage-labels.md) | Coding agents: intended mappings for five triage roles | 2026-09-13 |
| [../frontend/docs/product-analytics.md](../frontend/docs/product-analytics.md) | Frontend and privacy reviewers: optional analytics boundary and unverified external controls | 2026-09-13 |

## Evidence retained outside `docs/`

[`release-certification/authz-scope-investigation-2026-08-04.md`](../release-certification/authz-scope-investigation-2026-08-04.md)
stays at its original path because an integration test cites that exact path
(`backend/internal/handlers/authz_scope_integration_test.go:31-38`). The rest of
`release-certification/` contains the recent step-0 task prompts; its
[README](../release-certification/README.md) explains their evidentiary status.

The `audit/` sources used during this rebuild remain inspection inputs, not
current architecture documentation. In particular,
`audit/SYSTEM-AS-BUILT.md` describes the pre-remediation checkpoint; the current
payment index was added later by migration 40
(`backend/migrations/000040_one_non_terminal_payment_per_session.up.sql:1-29`).

To inspect a removed report without restoring it to the working tree:

```bash
git show docs-before-rebuild:path/to/file
```

