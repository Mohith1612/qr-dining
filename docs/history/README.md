# Archived documentation

**Historical record. Not operational instructions.**

Everything in this directory was true when it was written and is preserved because it explains how the system got here — the audits that drove the hardening phases, the plans that were executed, the reports that closed each gate. None of it is maintained.

**If anything here contradicts the guides in [`docs/`](../), the guides win.** Do not follow a procedure from this directory against a running system.

Common ways these files are stale: branch names (the RC lineage moved through `feature/certification-fixes-ui-redesign` and `premium-qr-collateral` before reaching `feature/signoz-observability`), commit SHAs, migration numbers (the schema is now v39), route counts, "open" findings that have since been fixed, and localhost URLs for stacks that no longer exist.

Current documentation index: [../../README.md](../../README.md).

---

## Why the system looks the way it does

| Document | What it explains |
|---|---|
| `operational-correctness-audit.md` | **The spine of everything since.** The 2026-05-21 audit that declared the prototype not production-ready, with six deployment blockers |
| `project-strategic-context-v1.md` | The engineering journey and the reasoning behind the major decisions. §13 lists findings that have since been fixed |
| `platform-hardening-architecture-plan.md` | The architecture plan for the platform/governance layer |
| `plans/` | Hardening phases 0–9, one plan per phase — guardrails, identity, RBAC, org model, platform trust domain, audit v2, realtime, payments, operational UX, final gate |

## Phase and readiness reports

`final-hardening-phase-a-report.md` · `final-hardening-phase-b-report.md` · `phase-c-behavioral-convergence-report.md` · `phase-d-staging-validation-report.md` · `phase-e-enforcement-readiness-report.md` · `final-operational-readiness-audit.md` · `final-production-readiness-assessment.md` · `final-release-readiness-report.md` · `final-rollout-gates-status.md` · `final-pilot-readiness-report.md` · `rollout-blocker-remediation-report.md` · `post-remediation-rollout-status.md` · `release-candidate-checklist.md` · `production-environment-checklist.md`

## Rollout and soak

`strict-rollout-plan-final.md` · `production-enforcement-rollout.md` · `r1-activation-runbook.md` · `r1-live-rollout-status.md` · `r1-soak-closure-report.md` · `r1-soak-monitoring-guide.md` · `manual-local-soak-operations-guide.md` · `staging-burnin-strategy.md`

> The R1 soak passed on 2026-06-04 over ~124 hours. It covered the **pre-redesign June binary**, not the current release candidate. "R1 is soaked" and "the pilot code is soaked" are different statements.

## Testing and validation

`playwright-behavioral-matrix.md` (the spec the 106 e2e tests were written from) · `e2e-failure-analysis.md` · `non-blocking-e2e-cleanup-plan.md` · `e2e-impact.md` · `chaos-test-results.md` · `pilot-load-validation-report.md` · `restore-verification-report.md` (schema 33) · `restore-verification-report-2026-09-12.md` (schema 40 — current; restore-and-replay failure, backup-path findings) · `manual-testing-findings-v1.md` … `v4` · `manual-testing-preparation-report.md` · `pilot-dress-rehearsal-report.md`

## Feature and capability records

`api-surface-certification-report.md` · `billing-ui-verification.md` · `staff-analytics-loyalty-summary-v1.md` · `shared-cart-implementation-plan-v1.md` · `promo-host-transfer-spec-v1.md` · `backend-workflow-stabilization-plan-v1.md` · `frontend-contract-stabilization.md` · `frontend-production-polish-plan-v1.md` · `redesign-handoff.md` · `operational-coordination-plan-v1.md` · `signoz-backend-instrumentation.md` (the OTel spec — since implemented)

## Session logs

`HANDOUT.md` — a superseded working session log. It describes the redesign as in progress; the redesign has since completed.
