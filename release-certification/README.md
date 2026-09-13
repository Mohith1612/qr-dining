# Retained release-certification evidence

This directory is not current product documentation. It contains three recent
step-0 task inputs plus one investigation that must remain at its exact path.
Use [the documentation index](../docs/README.md) for current behavior.

| File | Why it remains |
|---|---|
| [authz-scope-investigation-2026-08-04.md](authz-scope-investigation-2026-08-04.md) | Point-in-time investigation retained because the integration regression suite cites this exact path (`backend/internal/handlers/authz_scope_integration_test.go:31-38`). |
| [prompts/step0-a-e2e-quarantine.md](prompts/step0-a-e2e-quarantine.md) | Task input that defined the vacuity signatures and `test.fixme` annotation (`release-certification/prompts/step0-a-e2e-quarantine.md:46-105`). |
| [prompts/step0-b-go-toolchain-pin.md](prompts/step0-b-go-toolchain-pin.md) | Task input that required one explicit Go patch version across module, CI, and image build (`release-certification/prompts/step0-b-go-toolchain-pin.md:26-39`). |
| [prompts/step0-c-webhook-controls-and-census-correction.md](prompts/step0-c-webhook-controls-and-census-correction.md) | Task input that separated artifact generation from coverage and required a positive webhook control (`release-certification/prompts/step0-c-webhook-controls-and-census-correction.md:48-138`). |

These files describe the work requested at their point in time. Completion and
current behavior must be checked against code and the maintained docs; for
example, the resulting census is `audit/e2e-vacuity-census.md:1-42`, and the
pinned toolchain is `backend/go.mod:1-5` plus `.github/workflows/ci.yml:22-25`.

All other certification reports and screenshots were removed from the working
tree so agents cannot mistake them for current state. They remain available in
the `docs-before-rebuild` tag.
