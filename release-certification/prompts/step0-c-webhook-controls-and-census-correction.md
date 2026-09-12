# Task C — Restore webhook signature coverage, reclassify screenshot artifacts, fix the census format

Branch off `f8bccc4`. **Do not give this to whoever produced the census** — this task exists
because an adversarial pass found two defects in it, and the same author re-checking their own
work is how both got through the first time.

Cite this file and `audit/findings-ledger.md` alongside any F-number: a second, unrelated
F-numbering exists in `docs/history/operational-correctness-audit.md`.

## Why this exists

The quarantine commit was good work and survived an adversarial review substantially intact. Two
things did not survive.

**One.** Five `e2e/screenshots/sweep.spec.ts` declarations were graded KEEP and counted in the
66-declaration trusted-green figure. Three of them are `page.goto` followed by `page.screenshot`
with no locator and no assertion. `page.goto` resolves on any HTTP response, and a Next.js error
boundary renders with a 200 — so deleting the entire platform admin route leaves
`platform admin login page` green, capturing a PNG of the error page. The census's own evidence
column says "This is artifact coverage, not pilot invariant coverage" and then counted it as
coverage anyway.

**Two.** Seven structurally healthy declarations were left disabled under `test.describe.skip`
with the reason "merchant/provider settlement is unimplemented or out of scope under P5". All
seven test **webhook signature and timestamp verification**, which is not settlement. The route
is registered at `backend/internal/server/server.go:244` on the public API group behind a rate
limiter and **no auth middleware**. The HMAC check inside the handler is the only thing between
the internet and the payment webhook path, and it is live code whether or not settlement exists.
Out-of-scope functionality was used to justify disabling tests on an in-scope, unauthenticated,
publicly reachable security control.

## The invariant being enforced

A suite of one-sided rejection tests does not demonstrate that a control works. `X-04` ×3,
`W-03` ×2, `P-04` and `W-04` assert only `401` or a failure set; **not one of them asserts that a
valid request is accepted.** A handler that rejects every webhook unconditionally — unset provider
secret, misrouted path, verification failing closed for the wrong reason — passes all seven. A
control is only demonstrated when the same endpoint accepts the good input and rejects the bad
one.

## Plan before you implement, and expect to negotiate item 3

Post your plan and wait for approval. Item 3 is the one with real engineering risk; read it in
full before planning, because the obvious reading of it may be impossible.

---

## Item 1 — Reclassify the screenshot sweep

Introduce a fourth verdict, **`ARTIFACT`**: the declaration runs and produces something useful,
but asserts nothing about correctness and must not be counted as coverage.

All five `e2e/screenshots/sweep.spec.ts` declarations become `ARTIFACT`. Do **not** mark them
`test.fixme` — they should keep running, because the screenshots have value; they simply are not
evidence. Tag each with a greppable comment in the established form:

```ts
// ARTIFACT: produces screenshots; no assertion can fail on a broken UI. Not coverage.
```

Two of the five have some incidental strictness — the staff flow's `getByLabel("PIN").fill()`
throws if the control is missing, and the ended-session declaration asserts
`expect(closeRes.status).toBe(200)` on its own API staging. Classify all five `ARTIFACT` anyway.
The file's purpose is artifact capture, a split classification inside one sweep file will not
survive contact with phase 2b, and neither incidental assertion tests a pilot invariant.

**Correct the census header.** Structural KEEP drops from 73 to 68; executing trusted-green drops
from 66 to **61**. I expect exactly 61 before any webhook re-enable. If your count differs, say
why in the report rather than silently adjusting the figure to match mine.

---

## Item 2 — Split the webhook describes

Signature and timestamp verification runs; settlement behaviour stays skipped. Split each affected
`test.describe.skip` into two describes in the same file — an enabled one for verification, a
skipped one for settlement — rather than moving declarations between files.

Back on (7): `X-04` ×3, `W-03` ×2, `P-04` ×1, `W-04` ×1.
Staying skipped: `W-01`, `W-02`, `W-05`, `W-06`, `W-07`, `P-03`, `P-13` — except `W-01`, per item 3.

Keep the existing `UNIMPLEMENTED/OUT OF SCOPE (P5)` comment on the skipped describes and add, on
the enabled ones, a line saying why they are not covered by that scope decision: the route is
public and unauthenticated, so signature verification is live regardless of settlement.

**This item does not land on its own.** It is conditional on item 3.

---

## Item 3 — The positive control, which is a hard gate

Re-enabling seven one-sided rejection tests without a positive control ships the exact failure
mode this whole exercise exists to eliminate. So:

**If a positive control cannot be made to work, the seven stay skipped, and that becomes a
finding.** Writing that finding is a complete and successful outcome for this task. Do not ship
seven one-sided tests and describe it as progress.

### Read this before designing it

The obvious positive control — "a valid signed webhook settles the session", which is `W-01` —
may well be impossible, and for a good reason. Under the P5 fix,
`normalizePaymentMethodStatus` (`backend/internal/services/payment.go:759`) returns
`requires_staff_confirmation` for **every** payment method, so `provider_pending` is unreachable
from initiation and no payment ever carries a provider reference for a webhook to match. A
correctly signed webhook will therefore find nothing to settle. That is why `W-01` was quarantined
and why the settlement specs were skipped, and it is correct system behaviour, not a defect.

### The control you probably want instead

You do not need settlement to prove signature verification discriminates. You need **two requests
with identical bodies that differ only in their signature, and different outcomes**:

- correctly signed, referencing an unknown payment → the handler gets **past** signature
  verification and fails later, at provider/reference lookup;
- incorrectly signed, same body → rejected at signature verification with `401`.

A non-401 response to the correctly signed request is proof the request reached the lookup stage,
which is exactly the property the seven rejection tests cannot establish on their own — and it
needs no provider, no merchant and no settlement.

Determine the real status code for the valid-signature/unknown-reference path by reading
`paymentH.Webhook` and the service path behind it. **Assert it strictly.** Do not write a status
set. If you find yourself wanting `[200, 404, 422]`, you have not finished reading the handler,
and you are rebuilding the vacuity this task is correcting.

Repair `W-01` into this control, renaming its title to describe what it actually asserts. **This
is the only assertion change permitted anywhere in this task.**

### Prove the control is not itself vacuous

Two demonstrations, both output pasted verbatim into the report:

1. **Discrimination.** With the environment correctly configured, the positive control passes and
   all seven rejection tests pass. The endpoint accepts the good input and rejects the bad.
2. **Mutation.** Change the *client-side* secret in the positive control to a deliberately wrong
   value and show it **fails**. A positive control that stays green with a wrong secret is
   proving nothing. Revert the mutation before committing; show the red output in the report.

### Environment prerequisite

Confirm `PAYMENT_WEBHOOK_SECRET_STRIPE` is actually configured for the server the e2e suite runs
against. The specs fall back to the literal `"test-webhook-secret"`. If the server has no secret
configured for that provider, "wrong secret" and "no secret" are indistinguishable and `W-03`'s
second case is vacuous even after re-enabling. Report what you found and what you set. Changing
e2e-environment configuration is permitted; list every file you touched.

---

## Item 4 — Fix the evidence format for absence-based signatures

Phase 2b will use the census as a work queue, so the evidence column has to say what is missing,
not quote an unrelated line. All 8 `sig-5` rows currently quote an arbitrary setup line — `T-05`
quotes `const orgB = await seedOrg("t05b")`, which is not what makes it vacuous.

Rewrite every `sig-5` row in this form:

> No such line exists. The title requires an assertion that *&lt;the specific thing&gt;* is refused;
> the spec never issues the attempt. Under *&lt;named broken state&gt;* it still passes.

Verdicts do not change. This is a wording fix to eight rows.

---

## Hard constraints

You may modify: the webhook and adversarial spec files named above, `e2e/screenshots/sweep.spec.ts`
(comments only), `audit/e2e-vacuity-census.md`, and e2e-environment configuration.

You may **not**:
- Modify any file under `backend/` or `frontend/`. If the fix appears to require a production
  change, stop and report — that is a finding, not a licence.
- Change an assertion anywhere except in `W-01`.
- Re-enable any skipped declaration other than the seven named.
- Quarantine or un-quarantine anything else. The census's other verdicts were spot-checked
  adversarially and held; leave them alone.

## What I will check

- `git diff -U0 <base> HEAD -- e2e | grep -E '^[+-].*expect\('` shows changes in **`W-01` only**.
- `git diff --name-only` touches nothing under `backend/` or `frontend/`.
- The mutation demonstration is present and shows red output. Absent that, the re-enable is
  rejected regardless of how the suite looks.
- The corrected denominator is stated in the census header and its arithmetic is shown.

If item 3's gate fails, items 1, 2 and 4 still land — item 2 as the describe split with the
verification describes left skipped and a comment pointing at the finding. Say plainly in the
report which outcome you reached.
