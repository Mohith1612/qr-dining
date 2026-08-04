# Dependency upgrades

Dependabot proposes scheduled dependency upgrades after this repository is pushed to GitHub. Every Dependabot pull request is reviewed and merged manually; there is no auto-merge workflow.

## Update policy

- Security fixes are proposed immediately. In GitHub, enable **Settings → Advanced Security → Dependabot alerts** and **Dependabot security updates**. Grouped security updates may also be enabled there if desired; these repository settings are separate from `.github/dependabot.yml`.
- Patch updates are grouped and checked weekly where the ecosystem supports that cadence.
- Minor updates are grouped and delayed by cooldown so releases can soak upstream.
- Major updates are standalone pull requests and require explicit migration and release-note review.
- Never auto-merge database dependencies (`postgres` images, `golang-migrate`, and `pgx`), auth-adjacent dependencies such as `golang.org/x/crypto`, payment dependencies, framework majors (Gin, Next.js, React, and Astro), or infrastructure majors (SigNoz, ClickHouse, Redis, nginx, and Prometheus).

Every applicable upgrade pull request must be green on backend lint, unit tests with the race detector, the integration suite, `govulncheck`, sqlc drift, the OpenAPI lint, the migration up/down/up check, and the arm64 Docker build. Pull requests that touch frontend paths must also pass the frontend lint/build, marketing build, and e2e test-discovery checks. Full Playwright tests still do not run in CI and remain part of the manual multi-instance testing flow.

The frontend workflow is path-filtered by design, so it does not run on backend-only dependency pull requests. If branch protection is configured later, do not make these path-filtered checks globally required unless a no-op fallback is added. Required-check and branch-protection configuration remains a GitHub repository setting.

## Operational notes

- Dependabot runs on GitHub, not in a local checkout. After pushing, check **Insights → Dependency graph → Dependabot** for all five ecosystems, successful checks, expected labels, and expected grouping. Confirm a backend upgrade runs the main CI workflow (including migrations) and a frontend upgrade runs the frontend workflow.
- Before the first run, ensure the repository has `dependencies`, `go`, `javascript`, `ci`, `docker`, and `infrastructure` labels. Dependabot ignores configured labels that do not already exist.
- Enable Dependabot alerts and security updates in repository settings so known vulnerabilities produce immediate pull requests; the scheduled YAML configuration alone does not enable them.
- The OpenAPI lint command is pinned to `@redocly/cli@2.43.3`. Because it is pinned inside a `run:` step rather than declared in a manifest, Dependabot does not track it — bump it by hand.
- `golangci-lint` is pinned to `v2.12.2` on `golangci/golangci-lint-action@v8`. The action major is tracked by the GitHub Actions updater; the `version:` input is not, so bump it by hand.
- `govulncheck` is pinned to `v1.6.0` in the same way and has the same manual-bump caveat.

## Vulnerability scanning

The `Vulnerability Scan` job in `ci.yml` runs `govulncheck` against the backend module on every pull request.

- **It fails only on actionable findings.** govulncheck's default source mode exits non-zero only when a vulnerable symbol is genuinely reachable from this module's call graph. A vulnerable module that is required but never called exits 0 and stays informational, so the job does not generate noise that trains people to ignore it. No custom filtering or allowlist is used, and none should be added — an allowlist would silence the reachable findings that are the entire point.
- **The Go version is intentionally not pinned to an exact patch.** The job uses the same `"1.26"` spec as every other job and as `backend/docker/Dockerfile`. govulncheck reports standard-library findings against whatever toolchain it runs on, so a scanner pinned to a patch the release image does not use would report on a binary that is never shipped. Scanning this tree on go1.26.0 reports 19 standard-library vulnerabilities that do not exist in the shipped image, because CI and the Docker builder both resolve to go1.26.5.
- **The residual gap:** `setup-go`'s `"1.26"` and the Dockerfile's `golang:1.26-alpine` resolve independently. They agree today, but nothing enforces that. If they ever diverge, the scan silently stops describing the artifact. Deriving both from a single pinned patch version is the correct long-term fix and is not yet done.
- **Reproducing locally**, from `backend/`:

  ```bash
  go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
  ```

  Add `GOTOOLCHAIN=go1.26.5` if the local toolchain is older than the release toolchain, or the standard-library findings will not match CI.
- govulncheck covers Go only. The frontend, marketing, and e2e npm trees are not scanned in CI; use `npm audit --omit=dev` by hand and see the monitor list in `release-certification/dependency-security-report-2026-08-04.md`.
- GitHub reports any `dependabot.yml` parse errors on the repository's Dependabot page after push.
