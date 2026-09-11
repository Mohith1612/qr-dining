# Release certification

Evidence from the release-certification rounds. **This directory is a record of what was tested and found, not operational documentation.** For how to run or operate the system, start at [../README.md](../README.md).

---

## Current round — 2026-08-22

| File | What it is |
|---|---|
| [manual-certification-preflight-2026-08-22.md](manual-certification-preflight-2026-08-22.md) | **The authoritative current certification state.** Environment verification, what passed, what failed, suspected defects, and the full checklist being worked |
| [rc-manual-certification-plan.md](rc-manual-certification-plan.md) | Scope, limits and attention plan for the RC gate |

**Verdict: BLOCKED.** Exploratory functional testing may proceed; formal sign-off may not. Four blockers — candidate provenance, SigNoz ingestion failure, nine reachable advisories in the deployed binary, and no human Alertmanager receiver. Detail in the preflight; release context in [../docs/RELEASE.md](../docs/RELEASE.md).

The run sheet itself is the HTML suite in [../docs/manual-testing/](../docs/manual-testing/) — start at `testing-dashboard.html`.

## Standing investigations

| File | What it is |
|---|---|
| [authz-scope-investigation-2026-08-04.md](authz-scope-investigation-2026-08-04.md) | Cross-organization / cross-branch write bypass: investigation, fix and proof. **Cited directly by `backend/internal/handlers/authz_scope_integration_test.go` — do not move or rename** |

## Screenshots

`screenshots/` holds the guest-journey capture set from the 2026-07-20 round. Preflight evidence for the current round is under `e2e/screenshots/manual-certification-preflight-2026-08-22/` (git-ignored).

---

## Archive

Superseded rounds, preserved as evidence. **Every claim in these files was true when written and may not be true now.** Where an archived report contradicts the current guides, the guides win.

### `archive/2026-08-04/` — beta deployment, CI, and security round

| File | What it recorded |
|---|---|
| `independent-audit-2026-08-04.md` | Independent release audit. Verdict at the time: not ready to onboard Restaurant #1. Source of the authz blocker |
| `beta-deployment-report-2026-08-04.md` | Stand-up and validation of the beta environment on the shared VM |
| `beta-certification-readiness-2026-08-04.md` | Readiness verdict for that deployment. Explicitly superseded by the 2026-08-22 preflight |
| `ci-readiness-report-2026-08-04.md` | The first CI run the repository ever had — PR #1, 10 checks green |
| `dependency-security-report-2026-08-04.md` | `x/text`, `grpc` and Next.js advisories closed; `govulncheck` gate added |
| `release-readiness-report-2026-08-04.md` | Merge-readiness assessment |

### `archive/2026-07-18/` — RC automated certification round

| File | What it recorded |
|---|---|
| `automated-test-report.md` | Full automated verification sweep against the RC branch |
| `issues-found.md` | Issues raised during that certification |
| `fixed-during-certification.md` | Fixes made in response, commit by commit |
| `manual-testing-checklist.md` | The Markdown run sheet of that round, since replaced by the HTML suite |
| `release-summary.md` | Round summary and recommendation |

---

## Reading these safely

- **Branch names in archived reports are stale.** The RC lineage moved from `feature/certification-fixes-ui-redesign` to `feature/signoz-observability`; both are now ancestors of the trunk.
- **Commit SHAs are point-in-time** and generally no longer HEAD.
- **Environment URLs and credentials** in archived reports describe throwaway or beta environments. Live tester credentials belong in `docs/manual-testing/testing-dashboard.html`, not here.
- **"Green" in an archived report refers to that commit**, not to the current tree.
