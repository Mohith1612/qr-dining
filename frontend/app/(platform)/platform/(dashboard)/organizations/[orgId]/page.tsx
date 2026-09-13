"use client"

import { useCallback, useEffect, useState } from "react"
import { useParams } from "next/navigation"
import Link from "next/link"
import { usePlatformStore } from "@/store/platform"
import { platformApi } from "@/lib/api/platform"
import { hasPlatformRole } from "@/lib/platform-rbac"
import type { Organization, Branch, EffectiveEntitlements } from "@/types/platform"
import { HospitalityCard } from "@/components/shared/HospitalityCard"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
  PageHeader, Section, PlatformStatusBadge, PlatformLoading, ConfirmDialog,
} from "@/components/platform/ui"
import { ArrowLeft, Loader2 } from "lucide-react"
import { ApiError } from "@/lib/api/client"
import { toast } from "sonner"

export default function OrganizationDetailPage() {
  const params = useParams<{ orgId: string }>()
  const orgId = Number(params.orgId)
  const { token, roles } = usePlatformStore()
  const canManage = hasPlatformRole(roles)

  const [org, setOrg] = useState<Organization | null>(null)
  const [branches, setBranches] = useState<Branch[]>([])
  const [ent, setEnt] = useState<EffectiveEntitlements | null>(null)
  const [loading, setLoading] = useState(true)

  // Confirmation dialog state
  const [confirm, setConfirm] = useState<null | { kind: "org" | "branch"; action: "suspend" | "activate"; id: number; label: string }>(null)
  const [acting, setActing] = useState(false)

  // Branch edit modal state
  const [edit, setEdit] = useState<null | { id: number; name: string; code: string; timezone: string; logoUrl: string }>(null)
  const [savingEdit, setSavingEdit] = useState(false)

  async function openBranchEdit(b: Branch) {
    if (!token) return
    setEdit({ id: b.id, name: b.name, code: b.branch_code, timezone: b.timezone, logoUrl: "" })
    // Pull the current logo so an empty field doesn't clear it on save.
    try {
      const detail = await platformApi.getBranchDetail(b.id, token)
      setEdit((e) => e && e.id === b.id ? { ...e, logoUrl: detail.logo_url ?? "" } : e)
    } catch { /* leave logo blank */ }
  }

  async function saveBranchEdit() {
    if (!edit || !token) return
    if (edit.code.trim().length < 2) { toast.error("Branch code must be at least 2 characters."); return }
    setSavingEdit(true)
    try {
      await platformApi.updateBranch(edit.id, {
        name: edit.name.trim(),
        branch_code: edit.code.trim(),
        timezone: edit.timezone.trim(),
        logo_url: edit.logoUrl.trim(),
      }, token)
      toast.success("Branch updated.")
      setEdit(null)
      load()
    } catch (err) {
      toast.error(err instanceof ApiError && err.code === "BRANCH_CODE_EXISTS"
        ? "That branch code is already in use."
        : "Couldn't update the branch.")
    } finally {
      setSavingEdit(false)
    }
  }

  const load = useCallback(async () => {
    if (!token || !orgId) return
    setLoading(true)
    try {
      const [o, b, e] = await Promise.all([
        platformApi.getOrganization(orgId, token),
        platformApi.listBranches(orgId, token),
        platformApi.getOrganizationEntitlements(orgId, token),
      ])
      setOrg(o); setBranches(b.branches); setEnt(e.entitlements)
    } catch {
      toast.error("Couldn't load organization.")
    } finally {
      setLoading(false)
    }
  }, [token, orgId])

  useEffect(() => { load() }, [load])

  async function runConfirm() {
    if (!confirm || !token) return
    setActing(true)
    try {
      if (confirm.kind === "org") {
        if (confirm.action === "suspend") await platformApi.suspendOrganization(confirm.id, token)
        else await platformApi.activateOrganization(confirm.id, token)
      } else {
        if (confirm.action === "suspend") await platformApi.suspendBranch(confirm.id, token)
        else await platformApi.activateBranch(confirm.id, token)
      }
      toast.success(`${confirm.label} ${confirm.action === "suspend" ? "suspended" : "activated"}.`)
      setConfirm(null)
      await load()
    } catch {
      toast.error("Action failed.")
    } finally {
      setActing(false)
    }
  }

  if (loading) return <PlatformLoading />
  if (!org) return <p style={{ color: "var(--ink-3)" }}>Organization not found.</p>

  const capabilities = ent ? Object.entries(ent.capabilities).filter(([, v]) => v).map(([k]) => k) : []
  const limits = ent ? Object.entries(ent.limits) : []

  return (
    <div>
      <Link href="/platform/organizations" style={{ display: "inline-flex", alignItems: "center", gap: 6, color: "var(--ink-3)", fontSize: 13, textDecoration: "none", marginBottom: 14 }}>
        <ArrowLeft size={14} aria-hidden /> Organizations
      </Link>

      <PageHeader
        title={org.name}
        subtitle={`${org.code} · ${org.primary_contact_email || "no contact"}`}
        actions={
          <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
            <PlatformStatusBadge status={org.status} />
            {canManage && (org.status === "active" ? (
              <Button variant="destructive" size="sm" onClick={() => setConfirm({ kind: "org", action: "suspend", id: org.id, label: org.name })}>Suspend</Button>
            ) : (
              <Button variant="default" size="sm" onClick={() => setConfirm({ kind: "org", action: "activate", id: org.id, label: org.name })}>Activate</Button>
            ))}
          </div>
        }
      />

      <Section title="Plan & entitlements">
        <HospitalityCard elev={1} style={{ padding: "16px 18px", display: "flex", flexDirection: "column", gap: 12 }}>
          <div style={{ display: "flex", gap: 24, flexWrap: "wrap" }}>
            <Field label="Plan tier" value={ent?.plan_tier ?? "—"} />
            <Field label="Resolution source" value={ent?.source ?? "—"} />
          </div>
          <div>
            <span className="eyebrow">Capabilities</span>
            <div style={{ display: "flex", flexWrap: "wrap", gap: 6, marginTop: 8 }}>
              {capabilities.length === 0 ? <span style={{ color: "var(--ink-3)", fontSize: 13 }}>None</span> :
                capabilities.map((c) => (
                  <span key={c} className="mono" style={{ fontSize: 11, padding: "3px 8px", borderRadius: "var(--rad-pill)", background: "var(--accent-soft)", color: "var(--accent)" }}>{c}</span>
                ))}
            </div>
          </div>
          <div>
            <span className="eyebrow">Limits</span>
            <div style={{ display: "flex", flexWrap: "wrap", gap: 14, marginTop: 8 }}>
              {limits.map(([k, v]) => (
                <span key={k} style={{ fontSize: 13, color: "var(--ink-2)" }}>
                  <span className="mono" style={{ color: "var(--ink-4)" }}>{k}</span>{" "}
                  <strong style={{ color: "var(--ink-1)" }}>{v === -1 ? "∞" : v}</strong>
                </span>
              ))}
            </div>
          </div>
          <div style={{ display: "flex", gap: 18, flexWrap: "wrap" }}>
            <Link href={`/platform/entitlements?org=${org.id}`} style={{ fontSize: 13, color: "var(--accent)", textDecoration: "none" }}>
              Manage entitlements →
            </Link>
            <Link href={`/platform/organizations/${org.id}/billing`} style={{ fontSize: 13, color: "var(--accent)", textDecoration: "none" }}>
              Billing & subscription →
            </Link>
          </div>
        </HospitalityCard>
      </Section>

      <Section title={`Branches (${branches.length})`}>
        <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
          {branches.map((b) => (
            <HospitalityCard key={b.id} elev={1} style={{ padding: "12px 16px", display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12 }}>
              <div>
                <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
                  <span style={{ fontSize: 14, fontWeight: 600, color: "var(--ink-1)" }}>{b.name}</span>
                  <span className="mono" style={{ fontSize: 11, color: "var(--ink-4)" }}>{b.branch_code}</span>
                </div>
                <span style={{ fontSize: 12, color: "var(--ink-4)" }}>{b.timezone}</span>
              </div>
              <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
                <PlatformStatusBadge status={b.status} />
                {canManage && <Button variant="secondary" size="sm" onClick={() => openBranchEdit(b)}>Edit</Button>}
                {canManage && (b.status === "active" ? (
                  <Button variant="destructive" size="sm" onClick={() => setConfirm({ kind: "branch", action: "suspend", id: b.id, label: b.name })}>Suspend</Button>
                ) : (
                  <Button variant="default" size="sm" onClick={() => setConfirm({ kind: "branch", action: "activate", id: b.id, label: b.name })}>Activate</Button>
                ))}
              </div>
            </HospitalityCard>
          ))}
          {branches.length === 0 && <p style={{ color: "var(--ink-3)", fontSize: 14 }}>No branches.</p>}
        </div>
      </Section>

      <ConfirmDialog
        open={confirm !== null}
        onOpenChange={(v) => { if (!v) setConfirm(null) }}
        title={confirm ? `${confirm.action === "suspend" ? "Suspend" : "Activate"} ${confirm.kind}?` : ""}
        description={confirm ? `${confirm.action === "suspend" ? "Suspend" : "Activate"} “${confirm.label}”. This changes its lifecycle status.` : undefined}
        confirmLabel={confirm?.action === "suspend" ? "Suspend" : "Activate"}
        destructive={confirm?.action === "suspend"}
        loading={acting}
        onConfirm={runConfirm}
      />

      {edit && (
        <div
          onClick={() => !savingEdit && setEdit(null)}
          style={{ position: "fixed", inset: 0, background: "rgba(0,0,0,0.45)", zIndex: 60, display: "flex", alignItems: "center", justifyContent: "center", padding: 20 }}
        >
          <HospitalityCard elev={3} onClick={(e) => e.stopPropagation()} style={{ padding: 22, width: "100%", maxWidth: 440 }}>
            <h3 className="serif" style={{ fontSize: 18, fontWeight: 600, margin: "0 0 4px" }}>Edit branch</h3>
            <p style={{ fontSize: 12.5, color: "var(--ink-3)", margin: "0 0 16px" }}>Super-admin: rename, change the branch code, or set the tenant logo.</p>
            <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
              <label style={{ display: "flex", flexDirection: "column", gap: 5 }}>
                <span className="eyebrow" style={{ fontSize: 10 }}>Branch name</span>
                <Input value={edit.name} onChange={(e) => setEdit({ ...edit, name: e.target.value })} />
              </label>
              <label style={{ display: "flex", flexDirection: "column", gap: 5 }}>
                <span className="eyebrow" style={{ fontSize: 10 }}>Branch code</span>
                <Input value={edit.code} onChange={(e) => setEdit({ ...edit, code: e.target.value })} placeholder="SAFF-BND" />
              </label>
              <label style={{ display: "flex", flexDirection: "column", gap: 5 }}>
                <span className="eyebrow" style={{ fontSize: 10 }}>Timezone</span>
                <Input value={edit.timezone} onChange={(e) => setEdit({ ...edit, timezone: e.target.value })} />
              </label>
              <label style={{ display: "flex", flexDirection: "column", gap: 5 }}>
                <span className="eyebrow" style={{ fontSize: 10 }}>Logo URL</span>
                <Input value={edit.logoUrl} onChange={(e) => setEdit({ ...edit, logoUrl: e.target.value })} placeholder="https://…/logo.png" />
              </label>
            </div>
            <div style={{ display: "flex", gap: 8, marginTop: 18 }}>
              <Button onClick={saveBranchEdit} disabled={savingEdit}>
                {savingEdit ? <Loader2 className="animate-spin" style={{ width: 14, height: 14 }} /> : "Save"}
              </Button>
              <Button variant="ghost" onClick={() => setEdit(null)} disabled={savingEdit}>Cancel</Button>
            </div>
          </HospitalityCard>
        </div>
      )}
    </div>
  )
}

function Field({ label, value }: { label: string; value: string }) {
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <span className="eyebrow">{label}</span>
      <span style={{ fontSize: 14, color: "var(--ink-1)", textTransform: "capitalize" }}>{value}</span>
    </div>
  )
}
