# Dependency upgrades

Dependabot proposes scheduled dependency upgrades after this repository is pushed to GitHub. Every Dependabot pull request is reviewed and merged manually; there is no auto-merge workflow.

## Update policy

- Security fixes are proposed immediately. In GitHub, enable **Settings → Advanced Security → Dependabot alerts** and **Dependabot security updates**. Grouped security updates may also be enabled there if desired; these repository settings are separate from `.github/dependabot.yml`.
- Patch updates are grouped and checked weekly where the ecosystem supports that cadence.
- Minor updates are grouped and delayed by cooldown so releases can soak upstream.
- Major updates are standalone pull requests and require explicit migration and release-note review.
- Never auto-merge database dependencies (`postgres` images, `golang-migrate`, and `pgx`), auth-adjacent dependencies such as `golang.org/x/crypto`, payment dependencies, framework majors (Gin, Next.js, React, and Astro), or infrastructure majors (SigNoz, ClickHouse, Redis, nginx, and Prometheus).

Every applicable upgrade pull request must be green on backend lint, unit tests with the race detector, sqlc drift, the migration up/down/up check, and the arm64 Docker build. Pull requests that touch frontend paths must also pass the frontend lint/build, marketing build, and e2e test-discovery checks. The migration check plus race-enabled unit tests are the current practical stand-in for a separate integration-test suite; full Playwright tests remain part of the manual multi-instance testing flow.

The frontend workflow is path-filtered by design, so it does not run on backend-only dependency pull requests. If branch protection is configured later, do not make these path-filtered checks globally required unless a no-op fallback is added. Required-check and branch-protection configuration remains a GitHub repository setting.

## Operational notes

- Dependabot runs on GitHub, not in a local checkout. After pushing, check **Insights → Dependency graph → Dependabot** for all five ecosystems, successful checks, expected labels, and expected grouping. Confirm a backend upgrade runs the main CI workflow (including migrations) and a frontend upgrade runs the frontend workflow.
- Before the first run, ensure the repository has `dependencies`, `go`, `javascript`, `ci`, `docker`, and `infrastructure` labels. Dependabot ignores configured labels that do not already exist.
- Enable Dependabot alerts and security updates in repository settings so known vulnerabilities produce immediate pull requests; the scheduled YAML configuration alone does not enable them.
- The OpenAPI lint command, `npx @redocly/cli lint`, is unpinned and therefore invisible to Dependabot. Pinning it is a future hardening task.
- `golangci/golangci-lint-action` is tracked by the GitHub Actions updater, but its `version: latest` input is still unpinned. Pinning that tool version is a separate hardening task.
- GitHub reports any `dependabot.yml` parse errors on the repository's Dependabot page after push.
