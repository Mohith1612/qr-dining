"use client"

import { useCallback, useEffect, useState } from "react"
import { useParams } from "next/navigation"
import Link from "next/link"
import { usePlatformStore } from "@/store/platform"
import { platformApi } from "@/lib/api/platform"
import { hasPlatformRole } from "@/lib/platform-rbac"
import type {
  Organization,
  OrganizationSubscription,
  BillingProfile,
  SubscriptionInvoice,
  SubscriptionPayment,
  PlatformPlan,
} from "@/types/platform"
import { HospitalityCard } from "@/components/shared/HospitalityCard"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter, DialogClose,
} from "@/components/ui/dialog"
import {
  PageHeader, Section, PlatformStatusBadge, PlatformLoading, ConfirmDialog,
} from "@/components/platform/ui"
import { ArrowLeft, Loader2 } from "lucide-react"
import { toast } from "sonner"

type DialogKind =
  | "activate" | "trial" | "renew" | "cancel" | "changePlan"
  | "recordPayment" | "createInvoice"

const selectStyle: React.CSSProperties = {
  height: 32, borderRadius: "var(--rad-md)", border: "1px solid var(--line-2)",
  background: "var(--bg-elev-1)", color: "var(--ink-1)", padding: "0 10px", fontSize: 14, fontFamily: "inherit", width: "100%",
}

function toISO(dateStr: string): string | undefined {
  return dateStr ? new Date(dateStr + "T00:00:00Z").toISOString() : undefined
}

function fmtDate(iso: string | null): string {
  if (!iso) return "—"
  try {
    return new Date(iso).toLocaleDateString("en", { year: "numeric", month: "short", day: "numeric" })
  } catch {
    return iso
  }
}

function money(amount: string, currency: string): string {
  const n = Number(amount)
  if (Number.isNaN(n)) return `${amount} ${currency}`
  const sym = currency === "INR" ? "₹" : `${currency} `
  return `${sym}${n.toFixed(2)}`
}

export default function OrganizationBillingPage() {
  const params = useParams<{ orgId: string }>()
  const orgId = Number(params.orgId)
  const { token, roles } = usePlatformStore()
  const canManage = hasPlatformRole(roles, "billing_admin")

  const [org, setOrg] = useState<Organization | null>(null)
  const [sub, setSub] = useState<OrganizationSubscription | null>(null)
  const [profile, setProfile] = useState<BillingProfile | null>(null)
  const [invoices, setInvoices] = useState<SubscriptionInvoice[]>([])
  const [payments, setPayments] = useState<SubscriptionPayment[]>([])
  const [plans, setPlans] = useState<PlatformPlan[]>([])
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)

  const [dialog, setDialog] = useState<DialogKind | null>(null)
  const [confirm, setConfirm] = useState<null | { label: string; description: string; destructive?: boolean; run: () => Promise<void> }>(null)

  // form fields (shared across dialogs)
  const [planId, setPlanId] = useState<number | "">("")
  const [dateVal, setDateVal] = useState("")
  const [reason, setReason] = useState("")
  const [payMethod, setPayMethod] = useState("upi")
  const [payAmount, setPayAmount] = useState("")
  const [payReference, setPayReference] = useState("")
  const [payNotes, setPayNotes] = useState("")
  const [payInvoiceId, setPayInvoiceId] = useState<number | "">("")
  const [invAmount, setInvAmount] = useState("")
  const [invNotes, setInvNotes] = useState("")

  // billing profile edit
  const [editProfile, setEditProfile] = useState(false)
  const [pf, setPf] = useState<Partial<BillingProfile>>({})

  const load = useCallback(async () => {
    if (!token || !orgId) return
    setLoading(true)
    try {
      const [o, s, bp, inv, pay, pl] = await Promise.all([
        platformApi.getOrganization(orgId, token),
        platformApi.getSubscription(orgId, token),
        platformApi.getBillingProfile(orgId, token),
        platformApi.listInvoices(orgId, token),
        platformApi.listPayments(orgId, token),
        platformApi.listPlans(token),
      ])
      setOrg(o)
      setSub(s.subscription)
      setProfile(bp.billing_profile)
      setInvoices(inv.invoices)
      setPayments(pay.payments)
      setPlans(pl.plans)
    } catch {
      toast.error("Couldn't load billing.")
    } finally {
      setLoading(false)
    }
  }, [token, orgId])

  useEffect(() => { load() }, [load])

  function openDialog(kind: DialogKind) {
    setPlanId(sub?.plan_id ?? "")
    setDateVal(""); setReason("")
    setPayMethod("upi"); setPayAmount(""); setPayReference(""); setPayNotes(""); setPayInvoiceId("")
    setInvAmount(""); setInvNotes("")
    setDialog(kind)
  }

  async function runDialog() {
    if (!token || !dialog) return
    setBusy(true)
    try {
      switch (dialog) {
        case "activate":
          await platformApi.activateSubscription(orgId, { plan_id: planId === "" ? undefined : Number(planId), expires_at: toISO(dateVal) }, token)
          toast.success("Subscription activated.")
          break
        case "trial":
          if (!dateVal) { toast.error("Pick a trial end date."); setBusy(false); return }
          await platformApi.extendTrial(orgId, { plan_id: planId === "" ? undefined : Number(planId), trial_ends_at: toISO(dateVal)! }, token)
          toast.success(sub ? "Trial extended." : "Trial started.")
          break
        case "renew":
          await platformApi.renewSubscription(orgId, toISO(dateVal), token)
          toast.success("Subscription renewed.")
          break
        case "cancel":
          await platformApi.cancelSubscription(orgId, reason, token)
          toast.success("Subscription cancelled.")
          break
        case "changePlan":
          if (planId === "") { toast.error("Choose a plan."); setBusy(false); return }
          await platformApi.changeSubscriptionPlan(orgId, Number(planId), token)
          toast.success("Plan changed.")
          break
        case "recordPayment":
          if (!payAmount) { toast.error("Enter an amount."); setBusy(false); return }
          await platformApi.recordPayment(orgId, {
            method: payMethod, amount: payAmount, reference_number: payReference, notes: payNotes,
            received_at: toISO(dateVal), invoice_id: payInvoiceId === "" ? undefined : Number(payInvoiceId),
          }, token)
          toast.success("Payment recorded.")
          break
        case "createInvoice":
          if (!invAmount) { toast.error("Enter an amount."); setBusy(false); return }
          await platformApi.createInvoice(orgId, { amount: invAmount, due_date: toISO(dateVal), notes: invNotes }, token)
          toast.success("Invoice created.")
          break
      }
      setDialog(null)
      await load()
    } catch {
      toast.error("Action failed.")
    } finally {
      setBusy(false)
    }
  }

  async function saveProfile() {
    if (!token) return
    setBusy(true)
    try {
      const { billing_profile } = await platformApi.updateBillingProfile(orgId, pf, token)
      setProfile(billing_profile)
      setEditProfile(false)
      toast.success("Billing profile saved.")
    } catch {
      toast.error("Couldn't save billing profile.")
    } finally {
      setBusy(false)
    }
  }

  async function runConfirm() {
    if (!confirm) return
    setBusy(true)
    try {
      await confirm.run()
      setConfirm(null)
      await load()
    } catch {
      toast.error("Action failed.")
    } finally {
      setBusy(false)
    }
  }

  if (loading) return <PlatformLoading />
  if (!org) return <p style={{ color: "var(--ink-3)" }}>Organization not found.</p>

  const planLabel = sub?.plan_name ? `${sub.plan_name} (${sub.plan_tier})` : `plan #${sub?.plan_id ?? "—"}`

  return (
    <div>
      <Link href={`/platform/organizations/${orgId}`} style={{ display: "inline-flex", alignItems: "center", gap: 6, color: "var(--ink-3)", fontSize: 13, textDecoration: "none", marginBottom: 14 }}>
        <ArrowLeft size={14} aria-hidden /> {org.name}
      </Link>

      <PageHeader
        title="Billing & subscription"
        subtitle={`${org.code} · manual billing — subscription status is recorded and audited, not enforced`}
        actions={sub ? <PlatformStatusBadge status={sub.status} /> : undefined}
      />

      {/* ── Subscription ─────────────────────────────────────────────── */}
      <Section
        title="Subscription"
        actions={canManage ? (
          <div style={{ display: "flex", gap: 6, flexWrap: "wrap" }}>
            {!sub && <Button size="sm" variant="outline" onClick={() => openDialog("trial")}>Start trial</Button>}
            <Button size="sm" onClick={() => openDialog("activate")}>{sub ? "Activate / resume" : "Activate"}</Button>
            {sub && sub.status === "trial" && <Button size="sm" variant="outline" onClick={() => openDialog("trial")}>Extend trial</Button>}
            {sub && <Button size="sm" variant="outline" onClick={() => openDialog("renew")}>Renew</Button>}
            {sub && <Button size="sm" variant="outline" onClick={() => openDialog("changePlan")}>Change plan</Button>}
            {sub && sub.status !== "suspended" && sub.status !== "cancelled" && (
              <Button size="sm" variant="outline" onClick={() => setConfirm({ label: "Suspend subscription", description: "Mark this subscription suspended. This is recorded and audited but does not restrict the tenant yet.", run: () => platformApi.suspendSubscription(orgId, token!).then(() => { toast.success("Subscription suspended.") }) })}>Suspend</Button>
            )}
            {sub && sub.status !== "cancelled" && (
              <Button size="sm" variant="destructive" onClick={() => openDialog("cancel")}>Cancel</Button>
            )}
          </div>
        ) : undefined}
      >
        <HospitalityCard elev={1} style={{ padding: "16px 18px" }}>
          {!sub ? (
            <p style={{ color: "var(--ink-3)", fontSize: 14, margin: 0 }}>No subscription yet. {canManage ? "Activate one or start a trial." : ""}</p>
          ) : (
            <div style={{ display: "flex", flexWrap: "wrap", gap: 28 }}>
              <Field label="Status" value={sub.status} />
              <Field label="Plan" value={planLabel} />
              <Field label="Provider" value={sub.provider_type} />
              <Field label="Started" value={fmtDate(sub.started_at)} />
              <Field label="Trial ends" value={fmtDate(sub.trial_ends_at)} />
              <Field label="Expires" value={fmtDate(sub.expires_at)} />
              {sub.renewed_at && <Field label="Renewed" value={fmtDate(sub.renewed_at)} />}
              {sub.cancelled_at && <Field label="Cancelled" value={fmtDate(sub.cancelled_at)} />}
              {sub.cancellation_reason && <Field label="Reason" value={sub.cancellation_reason} />}
            </div>
          )}
        </HospitalityCard>
      </Section>

      {/* ── Billing profile ──────────────────────────────────────────── */}
      <Section
        title="Billing profile"
        actions={canManage && !editProfile ? (
          <Button size="sm" variant="outline" onClick={() => { setPf(profile ?? {}); setEditProfile(true) }}>Edit</Button>
        ) : undefined}
      >
        <HospitalityCard elev={1} style={{ padding: "16px 18px" }}>
          {editProfile ? (
            <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(220px, 1fr))", gap: 12 }}>
              <LabeledInput label="Business name" value={pf.business_name ?? ""} onChange={(v) => setPf({ ...pf, business_name: v })} />
              <LabeledInput label="GST number" value={pf.gst_number ?? ""} onChange={(v) => setPf({ ...pf, gst_number: v })} />
              <LabeledInput label="Tax identifier" value={pf.tax_identifier ?? ""} onChange={(v) => setPf({ ...pf, tax_identifier: v })} />
              <LabeledInput label="Billing email" value={pf.billing_email ?? ""} onChange={(v) => setPf({ ...pf, billing_email: v })} />
              <LabeledInput label="Billing contact" value={pf.billing_contact ?? ""} onChange={(v) => setPf({ ...pf, billing_contact: v })} />
              <LabeledInput label="Currency" value={pf.currency ?? "INR"} onChange={(v) => setPf({ ...pf, currency: v })} />
              <LabeledInput label="Billing address" value={pf.billing_address ?? ""} onChange={(v) => setPf({ ...pf, billing_address: v })} />
              <div style={{ gridColumn: "1 / -1", display: "flex", gap: 8, justifyContent: "flex-end" }}>
                <Button size="sm" variant="outline" onClick={() => setEditProfile(false)} disabled={busy}>Cancel</Button>
                <Button size="sm" onClick={saveProfile} disabled={busy}>{busy ? <Loader2 className="animate-spin" style={{ width: 14, height: 14 }} /> : null}Save</Button>
              </div>
            </div>
          ) : (
            <div style={{ display: "flex", flexWrap: "wrap", gap: 28 }}>
              <Field label="Business name" value={profile?.business_name || "—"} />
              <Field label="GST number" value={profile?.gst_number || "—"} />
              <Field label="Billing email" value={profile?.billing_email || "—"} />
              <Field label="Billing contact" value={profile?.billing_contact || "—"} />
              <Field label="Currency" value={profile?.currency || "INR"} />
              <Field label="Address" value={profile?.billing_address || "—"} />
            </div>
          )}
        </HospitalityCard>
      </Section>

      {/* ── Invoices ─────────────────────────────────────────────────── */}
      <Section
        title={`Invoices (${invoices.length})`}
        actions={canManage ? <Button size="sm" variant="outline" onClick={() => openDialog("createInvoice")}>New draft</Button> : undefined}
      >
        <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
          {invoices.length === 0 && <p style={{ color: "var(--ink-3)", fontSize: 14 }}>No invoices.</p>}
          {invoices.map((inv) => (
            <HospitalityCard key={inv.id} elev={1} style={{ padding: "12px 16px", display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, flexWrap: "wrap" }}>
              <div style={{ display: "flex", flexDirection: "column", gap: 2 }}>
                <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
                  <span className="mono" style={{ fontSize: 13, color: "var(--ink-1)", fontWeight: 600 }}>{inv.invoice_number}</span>
                  <PlatformStatusBadge status={inv.status} />
                </div>
                <span style={{ fontSize: 12, color: "var(--ink-4)" }}>
                  {money(inv.amount, inv.currency)} · issued {fmtDate(inv.issue_date)} · due {fmtDate(inv.due_date)}{inv.notes ? ` · ${inv.notes}` : ""}
                </span>
              </div>
              {canManage && (
                <div style={{ display: "flex", gap: 6 }}>
                  {inv.status === "draft" && <Button size="sm" variant="outline" onClick={() => setConfirm({ label: "Issue invoice", description: `Issue ${inv.invoice_number}.`, run: () => platformApi.issueInvoice(orgId, inv.id, token!).then(() => { toast.success("Invoice issued.") }) })}>Issue</Button>}
                  {(inv.status === "draft" || inv.status === "issued") && <Button size="sm" onClick={() => setConfirm({ label: "Mark invoice paid", description: `Mark ${inv.invoice_number} as paid.`, run: () => platformApi.markInvoicePaid(orgId, inv.id, token!).then(() => { toast.success("Invoice marked paid.") }) })}>Mark paid</Button>}
                  {(inv.status === "draft" || inv.status === "issued") && <Button size="sm" variant="destructive" onClick={() => setConfirm({ label: "Cancel invoice", description: `Cancel ${inv.invoice_number}.`, destructive: true, run: () => platformApi.cancelInvoice(orgId, inv.id, token!).then(() => { toast.success("Invoice cancelled.") }) })}>Cancel</Button>}
                </div>
              )}
            </HospitalityCard>
          ))}
        </div>
      </Section>

      {/* ── Manual payments ──────────────────────────────────────────── */}
      <Section
        title={`Manual payments (${payments.length})`}
        actions={canManage ? <Button size="sm" variant="outline" onClick={() => openDialog("recordPayment")}>Record payment</Button> : undefined}
      >
        <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
          {payments.length === 0 && <p style={{ color: "var(--ink-3)", fontSize: 14 }}>No payments recorded.</p>}
          {payments.map((p) => (
            <HospitalityCard key={p.id} elev={1} style={{ padding: "12px 16px", display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, flexWrap: "wrap" }}>
              <div style={{ display: "flex", flexDirection: "column", gap: 2 }}>
                <span style={{ fontSize: 14, color: "var(--ink-1)", fontWeight: 600 }}>{money(p.amount, p.currency)} · {p.method.replace("_", " ")}</span>
                <span style={{ fontSize: 12, color: "var(--ink-4)" }}>
                  {fmtDate(p.received_at)}{p.reference_number ? ` · ref ${p.reference_number}` : ""}{p.invoice_id ? ` · invoice #${p.invoice_id}` : ""}{p.notes ? ` · ${p.notes}` : ""}
                </span>
              </div>
            </HospitalityCard>
          ))}
        </div>
      </Section>

      {/* ── Action dialog ────────────────────────────────────────────── */}
      <Dialog open={dialog !== null} onOpenChange={(v) => { if (!v) setDialog(null) }}>
        <DialogContent showCloseButton={false}>
          <DialogHeader>
            <DialogTitle>{dialogTitle(dialog, !!sub)}</DialogTitle>
            <DialogDescription>{dialogDescription(dialog)}</DialogDescription>
          </DialogHeader>

          <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
            {(dialog === "activate" || dialog === "trial" || dialog === "changePlan") && (
              <Labeled label={dialog === "changePlan" ? "New plan" : "Plan"}>
                <select style={selectStyle} value={planId} onChange={(e) => setPlanId(e.target.value === "" ? "" : Number(e.target.value))}>
                  <option value="">{dialog === "activate" && sub ? "Keep current plan" : "Select a plan…"}</option>
                  {plans.map((p) => <option key={p.id} value={p.id}>{p.name} ({p.tier})</option>)}
                </select>
              </Labeled>
            )}
            {(dialog === "activate" || dialog === "renew") && (
              <Labeled label="Expires on"><input type="date" style={selectStyle} value={dateVal} onChange={(e) => setDateVal(e.target.value)} /></Labeled>
            )}
            {dialog === "trial" && (
              <Labeled label="Trial ends on"><input type="date" style={selectStyle} value={dateVal} onChange={(e) => setDateVal(e.target.value)} /></Labeled>
            )}
            {dialog === "cancel" && (
              <Labeled label="Reason"><Input value={reason} onChange={(e) => setReason(e.target.value)} placeholder="e.g. non-payment" /></Labeled>
            )}
            {dialog === "recordPayment" && (
              <>
                <Labeled label="Method">
                  <select style={selectStyle} value={payMethod} onChange={(e) => setPayMethod(e.target.value)}>
                    {["upi", "bank_transfer", "cash", "cheque", "other"].map((m) => <option key={m} value={m}>{m.replace("_", " ")}</option>)}
                  </select>
                </Labeled>
                <Labeled label="Amount"><Input value={payAmount} onChange={(e) => setPayAmount(e.target.value.replace(/[^0-9.]/g, ""))} inputMode="decimal" placeholder="1499.00" /></Labeled>
                <Labeled label="Reference number"><Input value={payReference} onChange={(e) => setPayReference(e.target.value)} placeholder="UTR / txn id" /></Labeled>
                <Labeled label="Received on"><input type="date" style={selectStyle} value={dateVal} onChange={(e) => setDateVal(e.target.value)} /></Labeled>
                <Labeled label="Against invoice (optional)">
                  <select style={selectStyle} value={payInvoiceId} onChange={(e) => setPayInvoiceId(e.target.value === "" ? "" : Number(e.target.value))}>
                    <option value="">None</option>
                    {invoices.map((inv) => <option key={inv.id} value={inv.id}>{inv.invoice_number} · {money(inv.amount, inv.currency)}</option>)}
                  </select>
                </Labeled>
                <Labeled label="Notes"><Input value={payNotes} onChange={(e) => setPayNotes(e.target.value)} /></Labeled>
              </>
            )}
            {dialog === "createInvoice" && (
              <>
                <Labeled label="Amount"><Input value={invAmount} onChange={(e) => setInvAmount(e.target.value.replace(/[^0-9.]/g, ""))} inputMode="decimal" placeholder="1499.00" /></Labeled>
                <Labeled label="Due on"><input type="date" style={selectStyle} value={dateVal} onChange={(e) => setDateVal(e.target.value)} /></Labeled>
                <Labeled label="Notes"><Input value={invNotes} onChange={(e) => setInvNotes(e.target.value)} placeholder="e.g. May subscription" /></Labeled>
              </>
            )}
          </div>

          <DialogFooter>
            <DialogClose render={<Button variant="outline" />}>Cancel</DialogClose>
            <Button variant={dialog === "cancel" ? "destructive" : "default"} disabled={busy} onClick={runDialog}>
              {busy ? <Loader2 className="animate-spin" style={{ width: 14, height: 14 }} /> : null}
              Confirm
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={confirm !== null}
        onOpenChange={(v) => { if (!v) setConfirm(null) }}
        title={confirm?.label ?? ""}
        description={confirm?.description}
        confirmLabel={confirm?.label ?? "Confirm"}
        destructive={confirm?.destructive}
        loading={busy}
        onConfirm={runConfirm}
      />
    </div>
  )
}

function dialogTitle(kind: DialogKind | null, hasSub: boolean): string {
  switch (kind) {
    case "activate": return hasSub ? "Activate / resume subscription" : "Activate subscription"
    case "trial": return hasSub ? "Extend trial" : "Start trial"
    case "renew": return "Renew subscription"
    case "cancel": return "Cancel subscription"
    case "changePlan": return "Change plan"
    case "recordPayment": return "Record manual payment"
    case "createInvoice": return "New draft invoice"
    default: return ""
  }
}

function dialogDescription(kind: DialogKind | null): string {
  switch (kind) {
    case "cancel": return "Cancellation is recorded and audited. It does not restrict the tenant in this phase."
    case "recordPayment": return "Internal bookkeeping record of a UPI / bank transfer / cash payment."
    default: return ""
  }
}

function Field({ label, value }: { label: string; value: string }) {
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <span className="eyebrow">{label}</span>
      <span style={{ fontSize: 14, color: "var(--ink-1)", textTransform: "capitalize" }}>{value}</span>
    </div>
  )
}

function Labeled({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label style={{ display: "flex", flexDirection: "column", gap: 5 }}>
      <span className="eyebrow">{label}</span>
      {children}
    </label>
  )
}

function LabeledInput({ label, value, onChange }: { label: string; value: string; onChange: (v: string) => void }) {
  return (
    <Labeled label={label}><Input value={value} onChange={(e) => onChange(e.target.value)} /></Labeled>
  )
}
