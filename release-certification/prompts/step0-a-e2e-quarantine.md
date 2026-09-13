# Task A — Quarantine the vacuous e2e specs and fix the denominator

You are working in `qr-dining` on a branch off the current head. This task produces
**one commit** that changes only `e2e/**/*.spec.ts` files and adds one report file.
Do not combine it with any other task on your plate.

## Why this exists

Our Playwright suite reports a large number of passing specs. A significant fraction
of those specs cannot fail — they are written so that a missing feature, an error
response, or an absent field produces a green result. The reported pass count is
therefore not evidence of anything, and we are about to gate a pilot on it.

We are not repairing those specs in this task. We are **labelling** them, so that the
green number becomes true today. Repair happens later, in invariant order.

## The invariant being enforced

A passing test must be capable of failing. Concretely: for every spec that reports
green, there must exist a change to production behaviour that would turn it red.
A spec for which no such change exists is not a test.

## Plan before you implement

Post your plan — the signature list you will match on, and the count of specs you
expect each signature to catch — and wait for approval before editing any file.

## Hard constraints

You may:
- Add `test.fixme(...)` or convert `test(` to `test.fixme(` on an existing declaration.
- Add comments.
- Add the one report file named below.

You may **not**, anywhere in this task:
- Modify, widen, narrow, or delete an existing assertion.
- Modify anything under `e2e/helpers/`, `e2e/global-setup.ts`, or `e2e/playwright.config.ts`.
- Modify any file outside `e2e/`. No production code, no migrations, no config, no `go.mod`.
- Delete a spec file or a test declaration.
- "Fix while you're in there." If you find a one-line repair, note it in the report
  and leave the code alone.

`git diff --stat` on your commit must show only `e2e/**/*.spec.ts` plus the report.
If it shows anything else, you have exceeded the task.

Use `test.fixme`, not `test.skip`. `fixme` means "this is known broken and must be
fixed"; `skip` means "not applicable here" and is how vacuous specs become permanent.

## Vacuity signatures

Match on structure, not on counts. Assertion count is explicitly **not** a signature:
`e2e/audit/A-04-waiter-cannot-read-audit.spec.ts` contains exactly one `expect()` and
is one of the better specs in the suite.

1. **Swallowed failure.** The action under test is wrapped in `.catch(() => [])`,
   `.catch(() => null)`, `.catch(() => ({}))`, or a `try/catch` that continues. The
   request failing is indistinguishable from it succeeding.
2. **Conditional assertion.** The assertion sits inside `if (entries.length > 0)`,
   `if (res.ok)`, `if (data)` or similar, so the absent-feature path asserts nothing.
   `e2e/audit/A-01-session-lifecycle-audited.spec.ts` is the reference case — its own
   trailing comment states the test passes when the endpoint is unimplemented.
3. **Disjunctive widening.** `expect(a || b).toBe(true)`, or a `toContain` over a
   status set that spans both success and failure (`[200, 201, 404]`). A set of
   several failure codes (`[401, 403]`) is acceptable; a set mixing success with
   failure is not.
4. **Assertion against a field the API does not return.** Compare the asserted field
   names against the response shape in `audit/SYSTEM-AS-BUILT.md` §2 or the handler.
   `undefined === undefined` is a vacuous pass.
5. **Claim without an attempt.** The title says "immutable", "cannot", "rejected", or
   "isolated", but the spec never issues the mutation or cross-tenant read it claims
   is refused.
6. **Fixture cannot reach the state in the title.** The spec name asserts behaviour in
   a state the fixture never constructs. `e2e/session/L-03` is the reference case: its
   title claims reconnect during `awaiting_reactivation`, but the fixture creates only
   an active session and the worker needs five minutes of durable absence to reach
   that state. It is currently red and is to be quarantined, **not** repaired by
   changing an expected status code — that would produce a vacuous green.
7. **UI claim tested by `fetch` only.** The title names a tab, device, reload, or
   reconnect, but the spec contains no `page.` interaction.
8. **Asserts the fixture, not the system.** The only assertions restate values the
   seed helper just supplied.

## What to produce

For every one of the spec files, a row in `audit/e2e-vacuity-census.md`:

| spec | declarations | verdict | signature | evidence | invariant |

- `verdict` is `KEEP`, `QUARANTINE`, or `UNREACHABLE` (signature 6 — fixture cannot
  reach the state; these are quarantined too but tracked separately, because repairing
  them needs fixture work, not assertion work).
- `evidence` is the **quoted line** that makes it vacuous plus one sentence naming a
  concrete broken-system state under which the spec still passes. "Looks weak" is not
  evidence and will be rejected.
- `invariant` is the identifier from `audit/Invariance.md` the spec is meant to cover
  (H1–H6, C1–C4, P1–P6, S1–S3, T1–T3), or `none`. Specs mapping to `none` are not
  pilot-gating and this is how we will know what to repair first.

Then apply `test.fixme` to every QUARANTINE and UNREACHABLE declaration, each with a
one-line comment in this exact form so it is greppable:

```ts
// VACUOUS(sig-2): asserts only when the audit endpoint returns rows; passes if it 404s.
test.fixme("session creation and close are audit-logged", async () => {
```

## The denominator

`audit/SYSTEM-AS-BUILT.md` says 129 declarations across 106 spec files. The working
tree has a different number. Your report opens with the real figures, each produced by
a command you quote in the report so I can re-run it:

- spec files on disk
- total `test(` / `test.fixme(` / `test.skip(` declarations
- declarations per Playwright project, and the per-project multiplier
- after your change: KEEP / QUARANTINE / UNREACHABLE counts, and KEEP broken down by
  mapped invariant vs `none`

State the KEEP count plainly as the number of specs we currently have grounds to
trust. I expect it to be far smaller than the number we have been quoting. Do not
soften that.

## What I will check when you hand this back

- `git diff --stat` touches nothing outside the allowed paths.
- Three QUARANTINE rows I pick at random: I will read the quoted line and confirm the
  spec passes under the broken state you describe.
- Three KEEP rows I pick at random: I will try to construct a broken system state
  under which they still pass. If I succeed, the census is not finished.
- No assertion anywhere in the diff has changed. `git diff -U0 -- e2e | grep -E '^[+-].*expect\('`
  must return nothing but context-free `test.fixme` reindentation, if anything.

If you believe a signature is wrong, or catches a spec it should not, say so in the
report rather than quietly declining to apply it.
