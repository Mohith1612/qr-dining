# Pilot Onboarding Checklist (Platform Operator)

Operator-driven onboarding of a new restaurant tenant. The platform UI orchestrates this
end-to-end at **`/platform/onboarding`** (super-admin). This checklist is the human-readable
companion — every step maps to a wizard step and a backend action that writes a
`platform_audit_log` row.

> Prereq: you are signed in to the Platform Control Plane as a **super_admin** operator
> (`/platform/login`). Org/branch creation is super-admin only.

## 1. Pre-call prep
- [ ] Confirm the deal: plan tier (free / standard / premium) and trial-vs-paid.
- [ ] Collect: restaurant name, desired slug, legal name, owner contact email.
- [ ] Collect billing: business/legal name, GST number, billing email.
- [ ] Collect operations: branch name, timezone, table count.

## 2. Run the wizard (`/platform/onboarding`)
1. **Organization & billing** — name, code (slug), restaurant slug, legal name, contact
   email; optional billing profile (business name, GST, billing email). → *Create organization*.
2. **Branch** — name, timezone, branch code (auto if blank), order prefix; optional **initial
   owner** (name, staff code, PIN) so the restaurant can log in to the staff app immediately.
   → *Create branch*.
3. **Plan** — select the agreed plan. → *Assign plan* (also syncs the entitlement resolver).
4. **Subscription** — Trial (set trial-end date) or Active (set expiry). → *Configure subscription*.
5. **Tables** — enter the table count; QR tokens are minted automatically (T1…Tn). → *Generate tables*.
6. **QR codes** — **Print sheet** (browser print / save-as-PDF) and/or **Download ZIP** of
   per-table PNGs. Hand these to the restaurant for table placement.
7. **Activate** — review the readiness summary, then **Activate tenant**. Status → "Restaurant ready".

## 3. Verify (post-wizard)
- [ ] Org detail (`/platform/organizations/:id`) shows the org **active** with the right plan.
- [ ] Billing (`…/billing`) shows the subscription (trial/active) + billing profile.
- [ ] Tables exist; scanning a printed QR opens the guest table page (`/table/<token>`).
- [ ] Give the owner their **staff code + PIN** and the staff app URL.
- [ ] (Optional) Set branding: **Themes** (`/platform/themes`) or have staff pick a theme in
      the admin app — both now write the structured `tenant_themes` source.

## 4. Hand-off
- [ ] Share the Restaurant Go-Live Checklist (`restaurant-go-live-checklist.md`) with the owner.
- [ ] Note the org id / branch code for support reference.

## Notes
- Every onboarding action is audited (`platform.organizations.create`, `platform.branches.create`,
  `platform.branches.tables.create`, `platform.org.plan.assign`, `platform.subscription.*`,
  `platform.organizations.activate`).
- The wizard is resumable in-session; if a step fails, fix the input and retry that step.
- Subscription **status is recorded and audited, not enforced** in this phase — an expired/suspended
  subscription does not yet restrict the tenant (see `phase-e-enforcement-readiness-report.md` and
  the Observability surface).
