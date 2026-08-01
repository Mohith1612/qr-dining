# Project Strategic Context — qr-dining (v1)

> Purpose:
> This document exists to preserve the *engineering journey, architectural intent,
> rollout philosophy, operational discipline, and implementation reasoning* behind
> the qr-dining system.
>
> The master-system-context-v1.md explains:
> - what the system currently IS
> - how the codebase works
>
> This document explains:
> - HOW the system evolved
> - WHY important decisions were made
> - WHAT engineering philosophy emerged
> - HOW future work should proceed safely
>
> This document is intended for:
> - future Claude sessions
> - future maintainers
> - future engineering phases
> - pilot/pre-production continuity
>
> It is NOT a README.
> It is NOT product documentation.
> It is strategic engineering continuity documentation.
>
> Status:
> Written during the late-stage stabilization + rollout period after R1 activation.

---

# 1. Project Evolution Summary

qr-dining did NOT begin as a hardened production-grade system.

The original system was:
- a fast-moving MVP
- operationally incomplete
- functionally rich
- but architecturally unsafe in multiple critical areas

The system initially:
- trusted frontend identity too heavily
- lacked strict lifecycle semantics
- had inconsistent branch isolation
- had weak realtime recovery guarantees
- had unsafe payment flows
- had incomplete operational correctness
- had insufficient rollout discipline

The project then evolved through:
- hardening phases
- operational audits
- lifecycle redesign
- rollout planning
- staged enforcement preparation
- frontend/backend contract stabilization
- collaborative dining semantic redesign
- manual operational walkthroughs

The system gradually transitioned from:
# “feature-heavy prototype”
to:
# “operationally credible restaurant system”

This transition is the single most important engineering story of the project.

---

# 2. The Core Product Philosophy

The system eventually converged on ONE fundamental product philosophy:

# The restaurant session is the primary unit of truth.

NOT:
- users
- accounts
- devices
- carts
- tabs

The SESSION is the central entity.

This led to several major architectural decisions:

- collaborative shared cart
- host-controlled ordering
- session-scoped realtime
- participant-scoped temporary identity
- waiter/kitchen operational separation
- backend-authoritative reconciliation
- lifecycle-centric state machines

The system intentionally behaves like:
# a real restaurant table

NOT:
# an ecommerce checkout app

This distinction is critically important.

Future changes should preserve this philosophy.

---

# 3. The Most Important Architectural Decisions

## 3.1 Shared Session Cart

Originally the system behaved closer to:
- independent participant ordering

This caused:
- operational ambiguity
- ordering chaos
- unclear ownership
- waiter confusion
- poor restaurant realism

The system evolved into:
# one shared cart per session

Any participant may:
- add/remove items

But only the HOST may:
- place the order
- initiate payment

This dramatically improved:
- operational clarity
- dining realism
- order coordination
- payment semantics

This is now considered a core invariant.

---

## 3.2 Host-Controlled Ordering

This became one of the most important semantic stabilizations.

Reasoning:
- restaurants need accountability
- multiple simultaneous submissions create chaos
- one table should behave as one operational unit

The host model was chosen because:
- it maps naturally to group dining
- it simplifies operational flows
- it improves kitchen/waiter coordination
- it avoids replay/duplicate-order risks

Important nuance:
- host gating is SERVER authoritative
- frontend gating is only UX

The backend is the final authority.

---

## 3.3 Backend-Authoritative State

One of the largest engineering improvements was the move toward:
# backend-authoritative recovery

Important philosophy:
- frontend state is disposable
- Redis state is ephemeral
- WebSocket delivery is not guaranteed
- Postgres is the authoritative source of truth

This led to:
- snapshot reconciliation
- authoritative reconnects
- replay-or-replace semantics
- idempotency
- immutable bill snapshots
- immutable audit logs

Future work should NEVER violate this philosophy.

Avoid:
- frontend-authoritative assumptions
- long-lived optimistic state
- non-reconcilable local state
- Redis-only correctness

---

## 3.4 Alert-Only Payment Escalation

One of the most important lifecycle decisions.

Question:
What happens when payment settlement stalls?

Several approaches were considered:
- auto-cancel
- auto-close
- reconciliation state
- alert-only

The chosen approach:
# alert-only escalation

Meaning:
- workers NEVER fabricate settlement
- workers NEVER auto-close sessions
- workers NEVER auto-cancel payments
- humans remain the settlement authority

This was chosen because:
- financial correctness > automation
- provider webhook races are dangerous
- fabricated settlement is unacceptable

This is a core operational invariant.

---

# 4. Hardening Philosophy

The project eventually adopted:
# rollout-first engineering discipline

Instead of:
- “ship immediately”
- “rewrite aggressively”
- “flip everything globally”

The system evolved into:
- feature-flagged enforcement
- staged rollout waves
- soak periods
- reversible deployment
- observability-gated rollout
- shadow enforcement before strict enforcement

This became a major maturity transition.

---

# 5. The Rollout Philosophy

The staged rollout architecture is one of the strongest parts of the system.

Important principles:
- every enforcement must be reversible
- every rollout must be observable
- every rollout must have metrics
- every rollout must have alerts
- every rollout must have rollback
- every rollout must have soak time

This is why:
- R1–R7 exist
- feature flags default false
- shadow enforcement exists
- rollout sequencing matters

This rollout discipline should NEVER be bypassed casually.

---

# 6. The Meaning of “Soak”

A major operational concept introduced late in development.

“Soak” means:
# running the real system continuously under realistic conditions WITHOUT changing semantics.

Purpose:
- validate stability over time
- validate operational behavior
- validate observability
- validate rollout assumptions
- surface long-tail failures
- confirm no regressions

Important soak discipline:
- do not reset soak DB
- do not invalidate rollout state
- do not mutate rollout semantics casually
- use isolated test DBs for testing
- preserve rollout flags during restarts

The soak period represented:
# operational confidence building

NOT:
# development downtime

---

# 7. The Evolution of Engineering Discipline

The project originally had:
- large uncontrolled edits
- unclear semantic ownership
- insufficient verification
- implementation-first thinking

Over time the workflow evolved into:
- phased implementation
- planning documents before coding
- verify-first debugging
- isolated test databases
- atomic commits
- clean git discipline
- rollout awareness
- operational caution

This evolution is extremely important.

Future work should preserve:
- small atomic commits
- no co-authors
- clean working tree
- planning before implementation
- verify-first investigation
- isolated DB testing
- no touching soak infrastructure

---

# 8. Frontend ↔ Backend Contract Stabilization

A major realization during stabilization:
many perceived “backend bugs” were actually:
# frontend contract mismatches

Examples:
- payment falsely showing success
- kitchen marking served
- menu availability “connection lost”
- reactivation flow mismatch
- shared session confusion

This led to:
# frontend-contract-stabilization.md

Key philosophy:
- backend semantics are authoritative
- frontend must align to backend lifecycle
- frontend should not invent state transitions

This dramatically improved system coherence.

---

# 9. Operational Workflow Realism

The system increasingly optimized for:
# real restaurant behavior

Several important operational decisions emerged:

## Kitchen
Kitchen:
- cooks
- prepares
- marks ready

Kitchen does NOT:
- serve
- settle payments

---

## Waiter
Waiter:
- serves
- acknowledges assistance
- settles payments
- coordinates guest operations

---

## Guest
Guests:
- collaborate
- build shared cart
- track order lifecycle
- initiate payment through host

---

This separation became a major stabilization milestone.

---

# 10. Realtime Philosophy

Important realization:
not every surface requires perfect realtime.

Guest realtime became:
- strong
- websocket-driven
- reconciliation-based

Staff realtime remained:
- partially polling-based
- operationally sufficient

A major architectural decision was:
# NOT introducing branch-level websocket channels prematurely.

Reason:
- avoid reopening architectural risk late-stage
- polling was operationally sufficient
- focus on pilot stability first

Future branch-level realtime may still happen later.

But only if:
- operational evidence justifies it

NOT:
- speculative engineering

---

# 11. Manual Testing Became Critically Important

Late-stage development revealed:
automated tests alone were insufficient.

Major operational flaws were discovered only through:
- multi-tab walkthroughs
- multi-role walkthroughs
- real interaction flows
- kitchen/waiter/guest coordination testing

This produced:
- manual-testing-findings-v1.md
- manual-testing-findings-v2.md
- eventual stabilization fixes

Important lesson:
# operational systems must be manually exercised.

---

# 12. Current System State (Strategically)

The system is now:
# operationally credible

NOT:
# experimental

Major architecture is stabilized:
- lifecycle
- payments
- auth
- tenancy
- realtime
- shared cart
- host authority
- audit
- rollout

Remaining work is mostly:
- frontend polish
- operational refinement
- deployment hardening
- pilot preparation
- observability refinement
- remaining rollout waves

This is a major milestone.

---

# 13. Current Known Strategic Risks

The remaining meaningful risks are:

## 13.1 Cross-Org Isolation Bug
The T-01 snapshot leak is the most serious remaining backend issue.

Must be fixed before real multi-tenant exposure.

---

## 13.2 Over-Engineering Risk
At this stage:
# unnecessary architecture work is dangerous.

Especially:
- websocket rewrites
- state rewrites
- lifecycle rewrites
- major refactors

The system is mature enough that:
stability matters more than architectural experimentation.

---

## 13.3 Pilot Reality Risk
Real restaurants may expose:
- operational workflow friction
- waiter coordination problems
- UX confusion
- kitchen ergonomics issues
- payment edge cases

Pilot feedback should guide:
future refinement priorities.

---

# 14. Development Philosophy Going Forward

Future implementation should follow:

## Preferred
- small scoped phases
- planning before coding
- verify-first debugging
- additive migrations
- isolated testing
- operational caution
- semantic preservation

## Avoid
- large uncontrolled rewrites
- speculative architecture
- touching stable lifecycle semantics
- violating rollout discipline
- changing invariants casually

---

# 15. Recommended Future Priorities

Likely next priorities:
- finish remaining frontend polish
- pilot deployment preparation
- deployment infrastructure hardening
- operational onboarding
- fix remaining e2e failures
- resolve cross-org isolation bug
- complete rollout waves carefully
- collect real pilot feedback

After pilot:
- observability refinement
- branch realtime improvements (if justified)
- scaling improvements
- audit hash-chain
- multi-instance soak validation

---

# 16. Important Workflow Rules For Future Claude Sessions

Future Claude sessions should:
- read BOTH context files first
- avoid premature architecture changes
- preserve semantic invariants
- preserve rollout discipline
- keep working tree clean
- use atomic commits
- avoid co-authors
- verify before changing
- never touch soak DB casually
- use isolated DBs for integration testing

When uncertain:
# prefer stability over cleverness.

---

# 17. Final Strategic Assessment

qr-dining evolved from:
# a promising but operationally unsafe prototype

into:
# a disciplined, lifecycle-driven, operationally credible realtime restaurant system.

The largest achievement was NOT:
- adding features

It was:
# stabilizing semantics.

Specifically:
- lifecycle correctness
- payment correctness
- operational correctness
- collaborative dining semantics
- rollout discipline
- backend-authoritative recovery

This maturity transition is the defining engineering story of the project.

The project is now approaching:
# pilot-readiness engineering

rather than:
# foundational architecture construction.

That is a major milestone.

---

# 18. Strategic Update (2026-05-29) — The SaaS Control-Plane Expansion

> Added after the platform-governance work on branch `platform-governance-entitlements`
> (unmerged). This section records WHY a new layer was started and the philosophy applied,
> consistent with §14's "small scoped phases / additive / preserve invariants" discipline.

## 18.1 The strategic shift
With the operational core stabilized (§§12, 17), the project began its **second arc**: from a
single operationally-credible restaurant system toward a **multi-tenant SaaS platform**. The new
work is *not* more restaurant features — it is the **control plane** an operator needs to govern
many tenants: plans & entitlements, organization/branch lifecycle, feature-flag targeting,
cross-tenant operational intelligence, and tenant branding. This is the natural continuation of
the "business scalability" intent: the restaurant domain is the product; the platform layer is
how it becomes a business.

## 18.2 What was built (one branch, four phases)
On `platform-governance-entitlements`, additively and in small atomic commits:
- **Entitlements** (org-level capabilities/limits) with a resolver that **bridges** the existing
  restaurant subscription model — new abstraction, zero regression.
- **Organization/branch lifecycle** (suspend/activate).
- **Feature-flag targeting** (global → org → branch), kept deliberately **separate** from the 9
  env strict-rollout flags so the rollout machinery's meaning is never diluted.
- **Platform analytics** — on-demand Postgres aggregation only (no warehouse/Kafka), honoring
  §13.2's anti-over-engineering stance.
- **Theme/branding** — structured, validated design tokens, then **adopted on the live guest UI**
  so platform-set branding actually drives the restaurant experience.
- **A Platform Control-Plane UI** inside the *same* Next.js app as a separate trust domain.

## 18.3 The philosophy carried forward
The platform layer was built with the **same discipline** that defined the maturity transition:
- **Additive over invasive.** New migrations/tables/routes only; no operational rewrite. The
  guest/staff domains, lifecycle, payments, websocket, and the R1 soak were untouched.
- **Shadow before strict — again.** Entitlements and feature flags are **resolve-only**: computed
  and metered, never yet enforced. Suspend/activate flips a status nothing reads. This mirrors
  the R-wave "shadow → soak → strict" rollout philosophy (§5): build the capability, observe it,
  enforce later behind a flag. Enforcement is a deliberate *future* decision, not a side effect.
- **Backend-authoritative, extended to branding.** The structured theme became the authoritative
  source; the frontend only *applies* it. Legacy `settings_json.theme` is **bridged**, not
  dropped — the same "no one loses what they had" instinct that shaped the lifecycle migrations.
- **Trust-boundary separation preserved.** The platform is its own trust domain end-to-end
  (auth, RBAC, routes, and now its own frontend route group + auth store) — never mixed with
  guest/staff.
- **Branch-first isolation.** All of it is branch-local and unmerged; `main` and the soak are
  unaffected until a deliberate, observable rollout.

## 18.4 Strategic risks / what to watch
- **Enforcement is the real cutover.** Today nothing enforces entitlements/flags/suspension.
  Turning enforcement on is the moment tenants can be *limited* — it must follow the same
  staged, reversible, observable discipline as R1–R7, almost certainly behind new flags.
- **Two theme write-paths.** The platform UI writes structured `tenant_themes`; the staff admin
  still writes legacy `settings_json.theme`. The read path is bridged, but the write paths should
  be unified before this is considered finished, to avoid operator confusion.
- **Don't let the control plane outrun the pilot.** Per §13.2/§13.3, the restaurant product still
  needs real pilot evidence. The platform layer is foundation for scale; it should not divert
  focus from proving the core operational loop with real restaurants.

## 18.5 Where the project now sits
Two parallel tracks: **(a)** finish the operational rollout (R1 soak → R2+ waves, pilot) — the
original arc; and **(b)** mature the **platform control plane** from resolve-only toward governed
enforcement — the new arc. Track (b) is intentionally inert until track (a)'s discipline is
applied to it. The defining story is unchanged: **stabilize semantics, then expand carefully** —
now extended from "operationally credible restaurant system" toward "governable multi-tenant
platform."

---

# 19. Strategic Update (2026-05-30) — Observability Before Control (Support Console)

> Added after the read-only Support Console (branch `platform-governance-entitlements`, unmerged).

## 19.1 Why this came next, and why read-only
Onboarding real restaurants creates an immediate operational need: when something looks wrong,
a platform operator must be able to **diagnose** it — find a session/order/payment, see its
state, participants, bill, webhooks, and audit trail — **without opening a psql shell**. Direct
DB access is the antithesis of this project's discipline (no audit trail, no RBAC, easy to
mutate by accident). The Support Console replaces that with a governed, observable surface.

Crucially, it was built as **observability, not control**: search + inspect only, **zero
mutations**. This is the same instinct that produced "shadow before strict" (§5) and
"alert-only payment escalation" (§3.4): *build the ability to see before the ability to act.*
An operator who can read everything but change nothing is the safe first rung — it earns
operational confidence and surfaces real workflows before any "fix it from the console" power
is contemplated.

## 19.2 The invariants it had to honor (and did)
- **Backend-authoritative, credential-safe.** A dedicated read path assembles sanitized
  aggregates from existing reads; it never reuses the guest-token snapshot and deliberately
  omits guest credentials (session token, device fingerprint) — a leak test enforces this.
- **Trust boundaries + RBAC.** Support reads are gated to support/auditor roles; billing is
  excluded. The console is the platform trust domain only.
- **Audit transparency.** Every read is logged; deep PII/financial reads emit a *tenant-visible*
  critical-risk audit row, so a restaurant can always see when platform looked at its data. This
  extends the project's "immutable, traceable" audit philosophy to operator activity itself.
- **Additive + branch-local.** No migration, no flags, no lifecycle/payment/websocket/authz
  change; soak untouched.

## 19.3 What to watch
- **The read→write temptation.** The obvious next ask is "let support *fix* it" (resettle a
  stuck payment, reopen a session). That crosses from observability into control and must be
  treated as a first-class, staged, reversible, heavily-audited capability — never a quiet
  addition of mutation buttons. The current zero-mutation guarantee is a feature; keep it
  explicit until such work is deliberately scoped.
- **PII exposure scales with tenants.** Read access to participant phones and bills is real PII;
  RBAC + mandatory audit are the current controls. Bulk/export was intentionally omitted and
  should stay gated.
- **Audit visibility depends on the R1 flag.** Tenant-visible support-access rows only persist
  when `AUDIT_LOG_V2_ENABLED` is on — another reason the R1 rollout (track a) underpins the
  platform layer's credibility.

## 19.4 The arc, refined
The platform layer now spans three postures: **govern** (entitlements/flags/lifecycle —
resolve-only today), **observe** (analytics + the Support Console — live), and **brand** (theme —
adopted). Observation shipped before governance enforcement *on purpose*: you watch the system
under real tenants first, then turn on control with the same R-wave discipline. "Stabilize
semantics, then expand carefully" now reads: **stabilize, then observe, then govern.**

---

# 20. Strategic Update (2026-05-30) — Pilot-Readiness Remediation (discipline under a deadline)

> Added after closing the three Pilot Dress Rehearsal findings on branch
> `pilot-readiness-remediation` (off `platform-governance-entitlements`, unmerged). This section
> records the *engineering posture* of the remediation, not its mechanics (those are in the master
> doc's 2026-05-30 update and `pilot-dress-rehearsal-report.md`).

## 20.1 Why this was a remediation, not a project
The dress rehearsal returned a **qualified GO**. That verdict framed everything that followed: the
job was not "make the system better" but "**close exactly the three findings and prove they are
closed**," with the **minimum possible change surface**. This is the discipline of §13.2
(over-engineering is dangerous at this stage) and §14 (small scoped phases) applied under the
gravity of a real pilot date — the temptation near a deadline is to either over-fix (redesign the
websocket/session/auth surfaces the findings touch) or under-fix (paper over symptoms). The phase
explicitly refused both: no websocket/payment/session/auth/rollout redesign, no new platform
features, no support-console mutations.

## 20.2 The invariants it honored
- **Backend-authoritative, credential-safe (§3.3).** F-8 was fixed at the response boundary by
  stripping a **dormant** credential (`session_token` — issued at creation but read by no auth path,
  declared-but-unused on the client) from *every* guest session response, not just the one endpoint
  the report named. The fix is **permanent and independent of the R6 rollout flag** — defense in
  depth that does not wait on, or weaken, the staged-flag machinery. This mirrors the Support
  Console's "sanitize at the read path" instinct (§19.2).
- **Frontend aligns to backend lifecycle (§8).** F-6 and F-1 were corrected **client-side** — the
  client now matches the backend's existing `{order:{…}}` event envelope and routes a failed
  ws-ticket through the backend's existing snapshot-reactivation path. No backend event format or
  lifecycle semantics changed; the frontend stopped inventing a dead-end the backend had already
  recovered from. Exactly the class of "perceived backend bug = frontend contract mismatch" §8 names.
- **Rollout discipline preserved (§5, §6).** R6 was *validated* (guest flows still work with
  `AUTH_GUEST_CREDENTIALS_REQUIRED=true`), not *flipped*; enablement timing stays an operational
  decision on the existing staged, reversible path. The R1 soak stack was never touched.

## 20.3 Verify-first, on isolated infrastructure
Every fix was proven on the isolated `pilot-validation` stack (`:8090`/`:8091`, throwaway
PG/Redis), never the soak — consistent with §6/§14's "isolated DBs for testing, never the soak
DB." Validation was **targeted, not a full re-rehearsal**: a snapshot security matrix
(no/valid/invalid/cross-session/cross-tenant tokens, under R6 off **and** on), the live order
tracker advancing without refresh, an idle-session reload recovering through ws-ticket
409→snapshot→201, and cross-instance propagation — each tied to the specific finding it closes.
The before/after evidence (the snapshot leak reproduced, then gone) is the §11 "manually exercise
operational systems" lesson applied surgically.

## 20.4 Where this leaves the pilot
The qualifier on the dress-rehearsal GO is removed: the operational loop, live guest order
tracking, and idle-reconnect recovery now all work end-to-end, with the guest credential no longer
exposed. **Pilot Restaurant #1 is ready.** The remaining items are deliberate *operational*
decisions, not engineering blockers — enable R6 before public multi-tenant exposure, run the pilot
with R1/audit on, and tune presence/reactivation aggressiveness (F-2) for real dining. The defining
story holds: **stabilize semantics, prove them, expand carefully** — and, near a pilot date, *fix
narrowly and prove it* rather than reopen settled surfaces.

---

# 21. Strategic Update (2026-05-30) — Premium QR Collateral (consume, don't duplicate)

> Added after building the Premium QR Collateral system on branch `premium-qr-collateral`
> (off `pilot-readiness-remediation`, unmerged). Mechanics live in the master doc's
> 2026-05-30 update and §16.12; this records the *engineering posture*.

## 21.1 Why this, why now
With the brand layer adopted on the live guest UI (§18.2/§16.6) and the pilot unblocked (§20),
the next natural step on the **brand** posture (§19.4's *govern / observe / brand*) was the
restaurant's most physical touchpoint: the QR on the table. The framing was explicit — this is
an **enhancement, not a redesign**: turn "download my QR codes" into "receive professional
hospitality branding collateral," **without** reopening onboarding, theme, branch, or billing
architecture. The goal is *feel* — upscale venues should get something that looks like print
collateral, not a utilitarian code — pursued with the same additive caution as everything since
the maturity transition.

## 21.2 The defining decision — consume the theme, never duplicate it
The single most important rule was **no second branding system**. Collateral *consumes*
`Theme (preset+tokens) + Branch metadata + Collateral config`; the theme remains the source of
truth for colour/typography, and collateral owns only the **physical** concern (format, text,
logo placement, WiFi, socials, exports). This is the same instinct as §18.3 ("backend-authoritative,
extended to branding; bridge, don't drop") applied one layer out: the renderer reuses the existing
`[data-theme]` token system verbatim (scoped to a subtree so a preview can show a theme other than
the operator's own console), so "the same format renders differently per theme" came for free. Had
this been built as its own colour/branding store, it would have created exactly the **two
write-paths** divergence §18.4 warns about — so the analogous trap here was avoided too: platform
and staff write **one shared `branch_collateral` row**, not two stores.

## 21.3 Discipline carried forward
- **Structured, validated, no arbitrary surface.** Config is a typed object with an allowlisted
  format set and length-capped fields — explicitly **no arbitrary HTML/CSS and no drag-and-drop
  editor**. This mirrors the theme layer's hex-only allowlisted tokens (§16.6): constrain the
  surface so it stays safe, on-brand, and reviewable. Unknown keys are rejected at the boundary.
- **Anti-over-engineering (§13.2).** No PDF/render infrastructure was introduced. Export reuses
  the browser's own print → Save-as-PDF and the *already-present* `jszip`/`qrcode.react` for a
  QR PNG/SVG + standalone `print.html` bundle — **zero new dependencies**. "Avoid unnecessary
  infrastructure" was a stated constraint and was honored.
- **Additive + branch-local + trust-boundary-clean.** One additive migration (000033), additive
  routes/UI; the platform studio and the staff tab share renderers and a single store but stay in
  their own trust domains; the existing staff single-card print path is left untouched. `main` and
  the soak are unaffected.

## 21.4 Verify-first, and why physical artifacts need eyes
Validation followed §6/§14: an isolated stack only (the backend rebuilt to a **fresh port** so a
pre-existing pilot process was never disturbed; the soak never touched), covering migration, API
CRUD/validation/isolation, the shared store across both trust domains, all five formats across
three themes, the export bundle, and a regression pass on the existing QR print. The sharpest
lesson reinforced §11 ("operational systems must be manually exercised"): the `table_tent` folded
**upside-down** — a defect invisible to types, build, and API tests, caught only by *looking at the
rendered artifact and reasoning about the physical fold*. Print/physical collateral especially must
be verified with a human eye, not just green checks.

## 21.5 Where it sits
This is a tenant-facing **premium capability on the brand arc** — foundation for how the product
*presents*, not an enforcement lever. It is intentionally additive and inert with respect to the
operational core and the rollout machinery. The defining story holds, now extended once more:
**stabilize semantics, then observe, then govern, then brand — each layer consuming what already
exists rather than duplicating it, and each proven on isolated infra before it touches anything
real.**

---

# 22. Strategic Update (2026-06-04) — R1 Soak Closure (certify what happened, don't relitigate it)

> Added after the forensic R1 soak-closure audit (`r1-soak-closure-report.md`). This records
> the *engineering posture* of the closure, not its mechanics (those are in the report and the
> master doc's 2026-06-04 update / §2.6 CP-5).

## 22.1 Why a closure audit, and why forensic
The R1 soak (track a — the operational rollout, §18.5) was the first place the "soak = build
operational confidence over time" philosophy (§6) met an inconvenient reality: the **host was
shut down unexpectedly mid-soak**. The disciplined response was *not* to shrug and restart, nor
to declare victory because "it had been running a while" — it was to **reconstruct, from primary
evidence, exactly what happened and for how long**, then issue a defensible verdict. This is the
verify-first instinct (§7, §11) applied to operations rather than code: an audit, read-only,
no restart, no infra mutation — honoring the "never touch the soak DB casually" rule (§6/§16)
even while certifying the soak's death.

## 22.2 The verdict, and why "PASS WITH OBSERVATIONS" is the honest altitude
The evidence was strong: **~124h continuous, zero crashes, zero `audit_write_failures`, a
graceful shutdown, and a clean no-recovery Postgres restart** — the 72h gate cleared with ~52h
of margin. The temptation was a clean "PASS." The audit declined it, because two things were
genuinely **unverified rather than proven**: the final ~75h ran **idle** (the process was alive
and workers ticked, but no traffic or health probes asserted health), and the **storage curve
at real volume** was never derived. Calling those "UNKNOWN" instead of folding them into a PASS
is the same discipline as **shadow-before-strict** (§5) and **observe-before-govern** (§19):
*don't claim more certainty than the evidence carries.* A verdict that over-states confidence is
its own form of the over-engineering risk (§13.2) — it invites the next wave to be chained on a
foundation that was only partly tested.

## 22.3 The deployment lesson that refused to stay learned
The proximate reason the app didn't survive the reboot is the **same defect flagged at CP-4**:
the binary was bind-mounted from `/tmp`, which the OS clears on boot, so the container came back
`Exited (127)`. CP-4 already named this ("the bind-from-`/tmp` deployment is not outage-resilient
— relocate the binary / add a liveness page before a real production soak"), and it **recurred**.
The strategic point is sharper than "fix the mount": a known operational risk that is *documented
but not remediated* will be **paid again, at a worse time**. Staging tolerated it twice; a pilot
or production soak would not. Deployment resilience (binary off `/tmp`, app-down alerting,
continuous external probing) is now a **precondition** for the production R1 soak, not a nice-to-have
— the first concrete item on the "operational decisions, not engineering blockers" list (§20.4).

## 22.4 Signal hygiene as an operational invariant
The soak logged **5346 `critical`** lines — all of them the **alert-only** `payment_pending`
escalations (§3.4) firing on ~20–28 stale manual-test sessions that never settled. The
alert-only invariant worked **exactly as designed** (it alerted forever and **never fabricated a
settlement or mutated state**) — a quiet confirmation of the §3.4 decision under days of pressure.
But the volume is the lesson: **stale test data manufacturing thousands of "critical" alerts would
mask a *real* stall** in production. "Clean the test data before the prod soak" is not tidiness;
it is preserving the **legibility of the alert channel** — the same instinct as §8's "frontend must
not invent state": the alert stream must not be allowed to cry wolf.

## 22.5 Where this leaves track (a)
R1's **engineering** question — *is the audit-v2 writer stable over a long run?* — is answered:
**yes**, with margin and zero defects. What remains is **operational**, not architectural:
re-run a *production-shaped* soak (hardened deploy, real traffic, continuous probing, real
volume) to convert the two UNKNOWNs into evidence, then flip R2 by deliberate decision — never
chained (§5). The defining story holds, now with an operations corollary: **stabilize semantics,
prove them, expand carefully — and when you certify an operational run, certify it on evidence,
name what you didn't test, and fix the deployment risk you already knew about before you ask it
to carry production.**

---

# 23. Strategic Update (2026-06-05) — Manual testing as the spec: fix the contract, don't redesign

> Added after a human manual-testing certification pass on the isolated 3-tenant stack and the
> first three fixes/features that came out of it (branch `premium-qr-collateral`, four atomic
> commits). Mechanics live in the master doc's 2026-06-05 update and `manual-testing-findings-v4.md`;
> this records the *engineering posture*.

## 23.1 Why this came next
With the operational core soaked (§22) and the brand/collateral arc shipped (§21), the product met
its next teacher: **a person actually using it**. §11 said it years-of-effort ago — "operational
systems must be manually exercised" — and a structured pass across three realistic tenants produced
a dozen findings. The strategic point is that almost none were deep architecture defects; they were
**frontend↔backend contract mismatches** (§8) — exactly the class the project has learned to expect.
The job was therefore *narrow correction*, not redesign: align the UI to the backend's existing
rules, or make a latent backend capability actually reachable.

## 23.2 What the three fixes have in common — leverage what already exists
- **Occupied-table check-in** was a *predicate* bug: the "joinable session" lookup was narrower than
  the "table is occupied" unique index, leaving a gap where a guest could neither join nor create.
  The fix widened one query to match the invariant the index already encodes and let `JoinSession`
  reuse the existing reactivation path — no new lifecycle, just internal consistency. This is the
  highest-value fix because it broke **every idled table**, not an exotic edge.
- **Host transfer** added a *manual* entry point to machinery that already existed end-to-end
  (`reassignHost` + the `HOST_CHANGED` realtime path, previously only auto-triggered). The realtime
  was already correct; only an authorized doorway was missing.
- **Promo per-guest limit** *activated* a dormant capability (`uses_per_phone` existed but was
  unenforceable because validate carried no phone and the form hardcoded the value). The fix made
  the cap reachable and honest, and — consistent with the user's own framing — gated it on a phone
  only where a limit is actually set, with explicit candor that an unverified phone is *soft*
  enforcement, not fraud-proof. "Build the capability, then enforce it deliberately" (§5) applied at
  feature scale.

Each change is additive, guest-scoped, and leaves payments, websocket transport, auth, and the
rollout flags untouched — the same discipline (§14) that has governed every layer since the maturity
transition.

## 23.3 Verify-first, including the human eye and the bisectable history
Every fix was proven on the isolated stack (API matrices + live browser walkthroughs via the
driver), never the soak — host transfer watched across two participants, the promo cap exercised to
its limit (2 redemptions → rejected) and the phone-gate reveal seen in the cart. The commits were
split into **four atomic, individually-building commits** so the history stays bisectable — the
"clean git discipline" of §7 made literal. This matters because the same pass left a backlog
(below); a reviewer must be able to reason about each change in isolation.

## 23.4 What was deliberately *not* done, and why that's the point
Nine findings remain open on purpose. Two are real correctness items to schedule next — the promo
**time-window timezone** bug (compared to the DB's UTC `LOCALTIME`, not the branch tz, so windowed
promos silently fail) and **moving promo entry to the bill** (which requires the payment path to
accept a promo, a payment-correctness change that earns its own scoped work, never a quiet rider on
a UI move). The rest are UX and a feature-gate/tenant-context cleanup. Fixing three and naming nine
is the §13.2 anti-over-engineering stance under a fresh pile of bug reports: **close what's
understood and verified, list what isn't, and refuse to let a manual-testing backlog become a
big-bang rewrite.** The defining story holds, now extended once more: **stabilize, observe, govern,
brand — and when real use surfaces friction, fix the contract narrowly, prove it, and keep the
history clean enough to trust.**

---

End of project-strategic-context-v1.md