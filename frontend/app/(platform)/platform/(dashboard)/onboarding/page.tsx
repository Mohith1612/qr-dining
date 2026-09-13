"use client"

import { useCallback, useEffect, useState } from "react"
import Link from "next/link"
import { usePlatformStore } from "@/store/platform"
import { platformApi } from "@/lib/api/platform"
import { hasPlatformRole } from "@/lib/platform-rbac"
import type { PlatformPlan, PlatformTable } from "@/types/platform"
import { HospitalityCard } from "@/components/shared/HospitalityCard"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { PageHeader } from "@/components/platform/ui"
import { QRPackage } from "@/components/platform/QRPackage"
import { Check, Loader2, ArrowRight } from "lucide-react"
import { toast } from "sonner"

const STEPS = ["Organization", "Branch", "Plan", "Subscription", "Tables", "QR codes", "Activate"]

const selectStyle: React.CSSProperties = {
  height: 32, borderRadius: "var(--rad-md)", border: "1px solid var(--line-2)",
  background: "var(--bg-elev-1)", color: "var(--ink-1)", padding: "0 10px", fontSize: 14, fontFamily: "inherit", width: "100%",
}

type Created = {
  orgId?: number
  orgName?: string
  restaurantSlug?: string
  branchId?: number
  branchName?: string
  planId?: number
  planLabel?: string
  subStatus?: string
  tables: PlatformTable[]
  activated?: boolean
}

function toISO(d: string): string | undefined {
  return d ? new Date(d + "T00:00:00Z").toISOString() : undefined
}

function errMsg(e: unknown, fallback: string): string {
  return e instanceof Error && e.message ? e.message : fallback
}

export default function OnboardingPage() {
  const { token, roles } = usePlatformStore()
  const canManage = hasPlatformRole(roles) // super_admin (org/branch creation is super-only)

  const [step, setStep] = useState(0)
  const [busy, setBusy] = useState(false)
  const [plans, setPlans] = useState<PlatformPlan[]>([])
  const [c, setC] = useState<Created>({ tables: [] })

  // form fields
  const [orgName, setOrgName] = useState("")
  const [orgCode, setOrgCode] = useState("")
  const [legalName, setLegalName] = useState("")
  const [contactEmail, setContactEmail] = useState("")
  const [restaurantSlug, setRestaurantSlug] = useState("")
  const [businessName, setBusinessName] = useState("")
  const [gstNumber, setGstNumber] = useState("")
  const [billingEmail, setBillingEmail] = useState("")

  const [branchName, setBranchName] = useState("")
  const [timezone, setTimezone] = useState("Asia/Kolkata")
  const [branchCode, setBranchCode] = useState("")
  const [orderPrefix, setOrderPrefix] = useState("")
  const [logoUrl, setLogoUrl] = useState("")
  const [ownerName, setOwnerName] = useState("")
  const [ownerCode, setOwnerCode] = useState("")
  const [ownerPin, setOwnerPin] = useState("")

  const [planId, setPlanId] = useState<number | "">("")
  const [subMode, setSubMode] = useState<"trial" | "active">("trial")
  const [subDate, setSubDate] = useState("")
  const [tableCount, setTableCount] = useState("8")

  const loadPlans = useCallback(async () => {
    if (!token) return
    try {
      const { plans } = await platformApi.listPlans(token)
      setPlans(plans)
    } catch { /* non-fatal */ }
  }, [token])
  useEffect(() => { loadPlans() }, [loadPlans])

  if (!canManage) {
    return (
      <div>
        <PageHeader title="Onboard a tenant" />
        <HospitalityCard elev={1} style={{ padding: 20 }}>
          <p style={{ color: "var(--ink-3)", fontSize: 14, margin: 0 }}>Onboarding requires a super-admin operator.</p>
        </HospitalityCard>
      </div>
    )
  }

  async function run(fn: () => Promise<void>, fallback: string) {
    if (!token) return
    setBusy(true)
    try { await fn() } catch (e) { toast.error(errMsg(e, fallback)) } finally { setBusy(false) }
  }

  async function createOrg() {
    await run(async () => {
      if (!orgName.trim() || !orgCode.trim() || !restaurantSlug.trim()) { toast.error("Name, code and restaurant slug are required."); return }
      const res = await platformApi.createOrganization({
        code: orgCode.trim(), name: orgName.trim(), legal_name: legalName.trim(),
        primary_contact_email: contactEmail.trim(), restaurant_slug: restaurantSlug.trim(), restaurant_name: orgName.trim(),
      }, token!)
      if (businessName.trim() || gstNumber.trim() || billingEmail.trim()) {
        await platformApi.updateBillingProfile(res.organization.id, {
          business_name: businessName.trim(), gst_number: gstNumber.trim(), billing_email: billingEmail.trim(),
        }, token!)
      }
      setC((p) => ({ ...p, orgId: res.organization.id, orgName: res.organization.name, restaurantSlug: res.restaurant.slug }))
      toast.success("Organization created.")
      setStep(1)
    }, "Couldn't create organization.")
  }

  async function createBranch() {
    await run(async () => {
      if (!branchName.trim()) { toast.error("Branch name is required."); return }
      // The owner block is optional, but if any field is filled all must be valid
      // (mirrors the backend: staff code ≥2 chars, PIN 4–8 digits) — otherwise the
      // server rejects it with a raw validation error.
      const ownerTouched = ownerName.trim() || ownerCode.trim() || ownerPin.trim()
      if (ownerTouched) {
        if (!ownerName.trim()) { toast.error("Owner name is required."); return }
        if (ownerCode.trim().length < 2) { toast.error("Owner staff code must be at least 2 characters."); return }
        if (ownerPin.trim().length < 4 || ownerPin.trim().length > 8) { toast.error("Owner PIN must be 4–8 digits."); return }
      }
      const owner = ownerTouched
        ? { name: ownerName.trim(), staff_code: ownerCode.trim(), pin: ownerPin.trim() } : undefined
      const res = await platformApi.createBranch(c.orgId!, {
        name: branchName.trim(), timezone: timezone.trim(), branch_code: branchCode.trim(), order_prefix: orderPrefix.trim(),
        logo_url: logoUrl.trim() || undefined,
        initial_owner: owner,
      }, token!)
      setC((p) => ({ ...p, branchId: res.branch.id, branchName: res.branch.name }))
      toast.success("Branch created.")
      setStep(2)
    }, "Couldn't create branch.")
  }

  async function assignPlan() {
    await run(async () => {
      if (planId === "") { toast.error("Choose a plan."); return }
      await platformApi.assignOrganizationPlan(c.orgId!, Number(planId), "active", token!)
      const label = plans.find((p) => p.id === Number(planId))
      setC((p) => ({ ...p, planId: Number(planId), planLabel: label ? `${label.name} (${label.tier})` : `plan #${planId}` }))
      toast.success("Plan assigned.")
      setStep(3)
    }, "Couldn't assign plan.")
  }

  async function configureSubscription() {
    await run(async () => {
      let status = ""
      if (subMode === "trial") {
        if (!subDate) { toast.error("Pick a trial end date."); return }
        const res = await platformApi.extendTrial(c.orgId!, { plan_id: c.planId, trial_ends_at: toISO(subDate)! }, token!)
        status = res.subscription.status
      } else {
        const res = await platformApi.activateSubscription(c.orgId!, { plan_id: c.planId, expires_at: toISO(subDate) }, token!)
        status = res.subscription.status
      }
      setC((p) => ({ ...p, subStatus: status }))
      toast.success("Subscription configured.")
      setStep(4)
    }, "Couldn't configure subscription.")
  }

  async function generateTables() {
    await run(async () => {
      const n = parseInt(tableCount, 10)
      if (!n || n < 1) { toast.error("Enter a table count."); return }
      const { tables } = await platformApi.createBranchTables(c.branchId!, { count: n }, token!)
      setC((p) => ({ ...p, tables }))
      toast.success(`${tables.length} tables generated.`)
      setStep(5)
    }, "Couldn't generate tables.")
  }

  async function activate() {
    await run(async () => {
      await platformApi.activateOrganization(c.orgId!, token!)
      if (c.subStatus !== "active") {
        const res = await platformApi.activateSubscription(c.orgId!, { plan_id: c.planId }, token!)
        setC((p) => ({ ...p, subStatus: res.subscription.status }))
      }
      setC((p) => ({ ...p, activated: true }))
      toast.success("Tenant activated.")
    }, "Couldn't activate tenant.")
  }

  return (
    <div>
      <PageHeader title="Onboard a tenant" subtitle="Operator-driven setup — organization → branch → plan → subscription → tables → QR → activate" />

      {/* step indicator */}
      <div style={{ display: "flex", flexWrap: "wrap", gap: 8, marginBottom: 22 }}>
        {STEPS.map((label, i) => {
          const done = i < step
          const active = i === step
          return (
            <div key={label} style={{ display: "flex", alignItems: "center", gap: 6, fontSize: 12, padding: "4px 10px", borderRadius: "var(--rad-pill)",
              background: active ? "var(--accent-soft)" : done ? "var(--ok-soft)" : "var(--line-2)",
              color: active ? "var(--accent)" : done ? "var(--ok)" : "var(--ink-3)", fontWeight: active ? 600 : 500 }}>
              {done ? <Check size={12} /> : <span style={{ opacity: 0.7 }}>{i + 1}</span>} {label}
            </div>
          )
        })}
      </div>

      <HospitalityCard elev={1} style={{ padding: "20px 22px" }}>
        {step === 0 && (
          <StepForm title="Organization & billing profile" onNext={createOrg} busy={busy} nextLabel="Create organization">
            <Grid>
              <LabeledInput label="Organization name *" value={orgName} onChange={setOrgName} placeholder="Maison Saffron" />
              <LabeledInput label="Code (slug) *" value={orgCode} onChange={setOrgCode} placeholder="maison-saffron" />
              <LabeledInput label="Restaurant slug *" value={restaurantSlug} onChange={setRestaurantSlug} placeholder="maison-saffron" />
              <LabeledInput label="Legal name" value={legalName} onChange={setLegalName} placeholder="Maison Saffron Pvt Ltd" />
              <LabeledInput label="Primary contact email" value={contactEmail} onChange={setContactEmail} placeholder="owner@example.com" />
            </Grid>
            <Divider label="Billing profile (optional)" />
            <Grid>
              <LabeledInput label="Business name" value={businessName} onChange={setBusinessName} />
              <LabeledInput label="GST number" value={gstNumber} onChange={setGstNumber} />
              <LabeledInput label="Billing email" value={billingEmail} onChange={setBillingEmail} />
            </Grid>
          </StepForm>
        )}

        {step === 1 && (
          <StepForm title="Branch" onNext={createBranch} busy={busy} nextLabel="Create branch" onBack={() => setStep(0)}>
            <Grid>
              <LabeledInput label="Branch name *" value={branchName} onChange={setBranchName} placeholder="Main Branch" />
              <LabeledInput label="Timezone" value={timezone} onChange={setTimezone} />
              <LabeledInput label="Branch code" value={branchCode} onChange={setBranchCode} placeholder="auto" />
              <LabeledInput label="Order prefix" value={orderPrefix} onChange={setOrderPrefix} placeholder="OR" />
            </Grid>
            <Divider label="Branding (optional)" />
            <Grid>
              <LabeledInput label="Logo URL" value={logoUrl} onChange={setLogoUrl} placeholder="https://…/logo.png" />
            </Grid>
            <Divider label="Initial owner (optional)" />
            <Grid>
              <LabeledInput label="Owner name" value={ownerName} onChange={setOwnerName} />
              <LabeledInput label="Staff code" value={ownerCode} onChange={setOwnerCode} />
              <LabeledInput label="PIN" value={ownerPin} onChange={setOwnerPin} />
            </Grid>
          </StepForm>
        )}

        {step === 2 && (
          <StepForm title="Plan" onNext={assignPlan} busy={busy} nextLabel="Assign plan" onBack={() => setStep(1)}>
            <Labeled label="Plan">
              <select style={selectStyle} value={planId} onChange={(e) => setPlanId(e.target.value === "" ? "" : Number(e.target.value))}>
                <option value="">Select a plan…</option>
                {plans.map((p) => <option key={p.id} value={p.id}>{p.name} ({p.tier}) · ₹{p.price_monthly}/mo</option>)}
              </select>
            </Labeled>
          </StepForm>
        )}

        {step === 3 && (
          <StepForm title="Subscription" onNext={configureSubscription} busy={busy} nextLabel="Configure subscription" onBack={() => setStep(2)}>
            <Labeled label="Mode">
              <select style={selectStyle} value={subMode} onChange={(e) => setSubMode(e.target.value as "trial" | "active")}>
                <option value="trial">Trial</option>
                <option value="active">Active (paid)</option>
              </select>
            </Labeled>
            <Labeled label={subMode === "trial" ? "Trial ends on *" : "Expires on"}>
              <input type="date" style={selectStyle} value={subDate} onChange={(e) => setSubDate(e.target.value)} />
            </Labeled>
          </StepForm>
        )}

        {step === 4 && (
          <StepForm title="Generate tables" onNext={generateTables} busy={busy} nextLabel="Generate tables" onBack={() => setStep(3)}>
            <Labeled label="Number of tables">
              <Input value={tableCount} onChange={(e) => setTableCount(e.target.value.replace(/[^0-9]/g, ""))} inputMode="numeric" />
            </Labeled>
            <p style={{ fontSize: 12, color: "var(--ink-4)", margin: 0 }}>Tables are named T1…Tn with QR tokens minted automatically.</p>
          </StepForm>
        )}

        {step === 5 && (
          <div>
            <StepHeading title={`QR codes (${c.tables.length} tables)`} />
            <p style={{ fontSize: 13, color: "var(--ink-3)", margin: "0 0 14px" }}>Print the sheet or download a ZIP of per-table QR PNGs for {c.branchName}.</p>
            <QRPackage tables={c.tables} slug={c.restaurantSlug} branchName={c.branchName ?? "branch"} />
            <div style={{ display: "flex", justifyContent: "space-between", marginTop: 20 }}>
              <Button variant="outline" onClick={() => setStep(4)}>Back</Button>
              <Button onClick={() => setStep(6)}>Continue <ArrowRight size={14} /></Button>
            </div>
          </div>
        )}

        {step === 6 && (
          <div>
            <StepHeading title={c.activated ? "Restaurant ready" : "Readiness summary"} />
            <div style={{ display: "flex", flexDirection: "column", gap: 8, marginBottom: 18 }}>
              <SummaryRow label="Organization" value={c.orgName} ok={!!c.orgId} />
              <SummaryRow label="Branch" value={c.branchName} ok={!!c.branchId} />
              <SummaryRow label="Plan" value={c.planLabel} ok={!!c.planId} />
              <SummaryRow label="Subscription" value={c.subStatus} ok={!!c.subStatus} />
              <SummaryRow label="Tables" value={`${c.tables.length} generated`} ok={c.tables.length > 0} />
            </div>
            {c.activated ? (
              <HospitalityCard elev={1} style={{ padding: 16, background: "var(--ok-soft)" }}>
                <p style={{ margin: 0, color: "var(--ok)", fontWeight: 600, fontSize: 14 }}>✓ {c.orgName} is live.</p>
                <div style={{ display: "flex", gap: 14, marginTop: 10 }}>
                  <Link href={`/platform/organizations/${c.orgId}`} style={{ fontSize: 13, color: "var(--accent)", textDecoration: "none" }}>View organization →</Link>
                  <Link href={`/platform/organizations/${c.orgId}/billing`} style={{ fontSize: 13, color: "var(--accent)", textDecoration: "none" }}>Billing →</Link>
                </div>
              </HospitalityCard>
            ) : (
              <div style={{ display: "flex", justifyContent: "space-between" }}>
                <Button variant="outline" onClick={() => setStep(5)}>Back</Button>
                <Button onClick={activate} disabled={busy}>{busy ? <Loader2 className="animate-spin" style={{ width: 14, height: 14 }} /> : null} Activate tenant</Button>
              </div>
            )}
          </div>
        )}
      </HospitalityCard>
    </div>
  )
}

function StepForm({ title, children, onNext, onBack, nextLabel, busy }: { title: string; children: React.ReactNode; onNext: () => void; onBack?: () => void; nextLabel: string; busy: boolean }) {
  return (
    <div>
      <StepHeading title={title} />
      <div style={{ display: "flex", flexDirection: "column", gap: 14 }}>{children}</div>
      <div style={{ display: "flex", justifyContent: "space-between", marginTop: 22 }}>
        {onBack ? <Button variant="outline" onClick={onBack} disabled={busy}>Back</Button> : <span />}
        <Button onClick={onNext} disabled={busy}>{busy ? <Loader2 className="animate-spin" style={{ width: 14, height: 14 }} /> : null} {nextLabel} <ArrowRight size={14} /></Button>
      </div>
    </div>
  )
}

function StepHeading({ title }: { title: string }) {
  return <h2 className="serif" style={{ fontSize: 19, fontWeight: 500, margin: "0 0 16px" }}>{title}</h2>
}

function Grid({ children }: { children: React.ReactNode }) {
  return <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(220px, 1fr))", gap: 12 }}>{children}</div>
}

function Divider({ label }: { label: string }) {
  return <div className="eyebrow" style={{ marginTop: 6, paddingTop: 12, borderTop: "1px solid var(--line-1)" }}>{label}</div>
}

function Labeled({ label, children }: { label: string; children: React.ReactNode }) {
  return <label style={{ display: "flex", flexDirection: "column", gap: 5 }}><span className="eyebrow">{label}</span>{children}</label>
}

function LabeledInput({ label, value, onChange, placeholder }: { label: string; value: string; onChange: (v: string) => void; placeholder?: string }) {
  return <Labeled label={label}><Input value={value} onChange={(e) => onChange(e.target.value)} placeholder={placeholder} /></Labeled>
}

function SummaryRow({ label, value, ok }: { label: string; value?: string; ok: boolean }) {
  return (
    <div style={{ display: "flex", alignItems: "center", gap: 10, fontSize: 14 }}>
      <span style={{ width: 16, height: 16, borderRadius: "50%", display: "inline-flex", alignItems: "center", justifyContent: "center", background: ok ? "var(--ok)" : "var(--line-2)", color: "#fff", flexShrink: 0 }}>
        {ok ? <Check size={11} /> : null}
      </span>
      <span style={{ color: "var(--ink-4)", minWidth: 110 }}>{label}</span>
      <span style={{ color: "var(--ink-1)", textTransform: "capitalize" }}>{value || "—"}</span>
    </div>
  )
}
