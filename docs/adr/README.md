# Architecture decision records

One file per significant, hard-to-reverse decision: the context, the choice, and why. An ADR is written once and then amended only with a dated superseding note — it is a record of what was decided at a point in time, not a living document.

| ADR | Decision | Date | Status |
|---|---|---|---|
| [0001](0001-signoz-adoption.md) | Adopt self-hosted SigNoz (Community Edition) for distributed tracing alongside the existing Prometheus stack | 2026-08-03 | Accepted — implemented, gated on `OTEL_ENABLED` |
| [0002](0002-r3-policy-decisions.md) | Ratify the three central-authz policy semantics that gate rollout wave R3 | 2026-05-29 | Accepted — all three ratified existing behaviour; R3 remains in shadow |
| [0003](0003-tenant-suspension-enforcement.md) | Enforce organization/branch suspension at guest entry only, leaving live sessions to finish | 2026-09-12 | Accepted — implemented, unflagged; one gap open (staff login lockout) |

## Notes on the current records

**0001** was written before the instrumentation existed and reads in the future tense ("no OpenTelemetry anywhere in the codebase"). OpenTelemetry **is** now implemented and `OTEL_ENABLED` defaults to `false`. The implementation spec is archived at [../history/signoz-backend-instrumentation.md](../history/signoz-backend-instrumentation.md); the phased rollout is [../signoz-rollout-runbook.md](../signoz-rollout-runbook.md).

**0002** closed the last hard blocker on R3 by ratifying existing behaviour, so no production behaviour change was required. R3 is still off, gated on 48 hours of zero shadow mismatches — see [../OPERATIONS.md §4](../OPERATIONS.md#4-rollout-flags-the-enforcement-ladder).

## Adding one

Number sequentially, name it `NNNN-short-slug.md`, and open with the date, the status, and the decision in one sentence. Record the alternatives you rejected and why — that is the part that stops the decision being relitigated.
