"use client"

import { useCallback, useEffect, useState } from "react"
import { usePlatformStore } from "@/store/platform"
import { platformApi } from "@/lib/api/platform"
import { hasPlatformRole } from "@/lib/platform-rbac"
import type { FeatureFlag, FlagState, Organization, Branch } from "@/types/platform"
import { HospitalityCard } from "@/components/shared/HospitalityCard"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter, DialogClose,
} from "@/components/ui/dialog"
import { PageHeader, Section, PlatformLoading } from "@/components/platform/ui"
import { Plus, Loader2 } from "lucide-react"
import { toast } from "sonner"

const SOURCE_TONE: Record<string, string> = {
  branch_override: "var(--accent)",
  org_override: "var(--info)",
  global_override: "var(--warn)",
  default: "var(--ink-4)",
}

export default function FeatureFlagsPage() {
  const { token, roles } = usePlatformStore()
  const canManage = hasPlatformRole(roles)

  const [flags, setFlags] = useState<FeatureFlag[]>([])
  const [orgs, setOrgs] = useState<Organization[]>([])
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)

  // targeting
  const [orgId, setOrgId] = useState<number | undefined>(undefined)
  const [branches, setBranches] = useState<Branch[]>([])
  const [branchId, setBranchId] = useState<number | undefined>(undefined)
  const [orgFlags, setOrgFlags] = useState<FlagState[]>([])
  const [branchFlags, setBranchFlags] = useState<FlagState[]>([])

  // create dialog
  const [createOpen, setCreateOpen] = useState(false)
  const [nf, setNf] = useState({ key: "", name: "", description: "", default_enabled: false })

  const loadFlags = useCallback(async () => {
    if (!token) return
    const { flags } = await platformApi.listFlags(token)
    setFlags(flags)
  }, [token])

  useEffect(() => {
    if (!token) return
    Promise.all([loadFlags(), platformApi.listOrganizations(token).then((r) => setOrgs(r.organizations))])
      .catch(() => toast.error("Couldn't load flags."))
      .finally(() => setLoading(false))
  }, [token, loadFlags])

  const loadOrgScope = useCallback(async () => {
    if (!token || !orgId) { setOrgFlags([]); setBranches([]); return }
    const [f, b] = await Promise.all([
      platformApi.getOrgFlags(orgId, token),
      platformApi.listBranches(orgId, token),
    ])
    setOrgFlags(f.flags)
    setBranches(b.branches)
  }, [token, orgId])

  useEffect(() => { loadOrgScope() }, [loadOrgScope])

  const loadBranchScope = useCallback(async () => {
    if (!token || !branchId) { setBranchFlags([]); return }
    const f = await platformApi.getBranchFlags(branchId, token)
    setBranchFlags(f.flags)
  }, [token, branchId])

  useEffect(() => { loadBranchScope() }, [loadBranchScope])

  async function run(fn: () => Promise<unknown>, after: () => Promise<void>) {
    setBusy(true)
    try { await fn(); await after(); toast.success("Updated.") }
    catch { toast.error("Action failed.") }
    finally { setBusy(false) }
  }

  async function createFlag() {
    if (!token || !nf.key.trim() || !nf.name.trim()) return
    setBusy(true)
    try {
      await platformApi.createFlag({ key: nf.key.trim(), name: nf.name.trim(), description: nf.description, default_enabled: nf.default_enabled }, token)
      toast.success("Flag created.")
      setCreateOpen(false)
      setNf({ key: "", name: "", description: "", default_enabled: false })
      await loadFlags()
    } catch {
      toast.error("Couldn't create flag (key may already exist).")
    } finally {
      setBusy(false)
    }
  }

  if (loading) return <PlatformLoading />

  return (
    <div>
      <PageHeader
        title="Feature Flags"
        subtitle="Boolean targeting — precedence: branch > org > global > default"
        actions={canManage ? <Button onClick={() => setCreateOpen(true)}><Plus size={14} aria-hidden /> New flag</Button> : undefined}
      />

      <Section title="Catalog & global overrides">
        <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
          {flags.map((f) => (
            <HospitalityCard key={f.key} elev={1} style={{ padding: "14px 18px", display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, flexWrap: "wrap" }}>
              <div style={{ minWidth: 0 }}>
                <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
                  <span style={{ fontSize: 14, fontWeight: 600, color: "var(--ink-1)" }}>{f.name}</span>
                  <span className="mono" style={{ fontSize: 11, color: "var(--ink-4)" }}>{f.key}</span>
                </div>
                <span style={{ fontSize: 12, color: "var(--ink-4)" }}>
                  default {f.default_enabled ? "on" : "off"}
                  {f.global_override != null && <> · global override <strong style={{ color: "var(--ink-2)" }}>{f.global_override ? "on" : "off"}</strong></>}
                </span>
              </div>
              {canManage && (
                <div style={{ display: "flex", gap: 6 }}>
                  <Button variant="outline" size="sm" onClick={() => run(() => platformApi.setGlobalFlag(f.key, true, token!), loadFlags)} disabled={busy}>Global on</Button>
                  <Button variant="outline" size="sm" onClick={() => run(() => platformApi.setGlobalFlag(f.key, false, token!), loadFlags)} disabled={busy}>Global off</Button>
                  <Button variant="ghost" size="sm" onClick={() => run(() => platformApi.clearGlobalFlag(f.key, token!), loadFlags)} disabled={busy || f.global_override == null}>Clear</Button>
                </div>
              )}
            </HospitalityCard>
          ))}
          {flags.length === 0 && <p style={{ color: "var(--ink-3)", fontSize: 14 }}>No flags yet.</p>}
        </div>
      </Section>

      <Section title="Targeted overrides">
        <div style={{ display: "flex", gap: 12, flexWrap: "wrap", marginBottom: 12 }}>
          <label style={{ display: "flex", flexDirection: "column", gap: 6, minWidth: 220 }}>
            <span className="eyebrow">Organization</span>
            <SelectBox value={orgId ?? ""} onChange={(v) => { setOrgId(v ? Number(v) : undefined); setBranchId(undefined) }}>
              <option value="">Select organization…</option>
              {orgs.map((o) => <option key={o.id} value={o.id}>{o.name} ({o.code})</option>)}
            </SelectBox>
          </label>
          {orgId && (
            <label style={{ display: "flex", flexDirection: "column", gap: 6, minWidth: 220 }}>
              <span className="eyebrow">Branch (optional)</span>
              <SelectBox value={branchId ?? ""} onChange={(v) => setBranchId(v ? Number(v) : undefined)}>
                <option value="">— org level —</option>
                {branches.map((b) => <option key={b.id} value={b.id}>{b.name} ({b.branch_code})</option>)}
              </SelectBox>
            </label>
          )}
        </div>

        {orgId && !branchId && (
          <ResolvedTable
            title="Resolved at organization"
            rows={orgFlags}
            canManage={canManage}
            onSet={(key, enabled) => run(() => platformApi.setOrgFlag(orgId, key, enabled, "platform console", token!), loadOrgScope)}
            onClear={(key) => run(() => platformApi.clearOrgFlag(orgId, key, token!), loadOrgScope)}
            busy={busy}
            clearableSource="org_override"
          />
        )}

        {orgId && branchId && (
          <ResolvedTable
            title="Resolved at branch"
            rows={branchFlags}
            canManage={canManage}
            onSet={(key, enabled) => run(() => platformApi.setBranchFlag(branchId, key, enabled, "platform console", token!), loadBranchScope)}
            onClear={(key) => run(() => platformApi.clearBranchFlag(branchId, key, token!), loadBranchScope)}
            busy={busy}
            clearableSource="branch_override"
          />
        )}
      </Section>

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>New feature flag</DialogTitle>
            <DialogDescription>Create a flag in the catalog. Keys are immutable.</DialogDescription>
          </DialogHeader>
          <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
            <Field label="Key"><Input value={nf.key} onChange={(e) => setNf((s) => ({ ...s, key: e.target.value }))} placeholder="new_checkout_flow" /></Field>
            <Field label="Name"><Input value={nf.name} onChange={(e) => setNf((s) => ({ ...s, name: e.target.value }))} placeholder="New Checkout Flow" /></Field>
            <Field label="Description"><Input value={nf.description} onChange={(e) => setNf((s) => ({ ...s, description: e.target.value }))} /></Field>
            <label style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 13, color: "var(--ink-2)" }}>
              <input type="checkbox" checked={nf.default_enabled} onChange={(e) => setNf((s) => ({ ...s, default_enabled: e.target.checked }))} style={{ width: 16, height: 16, accentColor: "var(--accent)" }} />
              Enabled by default
            </label>
          </div>
          <DialogFooter>
            <DialogClose render={<Button variant="outline" />}>Cancel</DialogClose>
            <Button onClick={createFlag} disabled={busy || !nf.key.trim() || !nf.name.trim()}>
              {busy ? <Loader2 className="animate-spin" style={{ width: 14, height: 14 }} /> : null} Create
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

function ResolvedTable({
  title, rows, canManage, onSet, onClear, busy, clearableSource,
}: {
  title: string
  rows: FlagState[]
  canManage: boolean
  onSet: (key: string, enabled: boolean) => void
  onClear: (key: string) => void
  busy: boolean
  clearableSource: string
}) {
  return (
    <HospitalityCard elev={1} style={{ padding: "8px 0" }}>
      <div style={{ padding: "6px 18px" }}><span className="eyebrow">{title}</span></div>
      {rows.map((f) => (
        <div key={f.key} style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, padding: "10px 18px", borderTop: "1px solid var(--line-1)", flexWrap: "wrap" }}>
          <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
            <span style={{ width: 8, height: 8, borderRadius: "50%", background: f.enabled ? "var(--ok)" : "var(--ink-4)" }} />
            <span className="mono" style={{ fontSize: 13, color: "var(--ink-1)" }}>{f.key}</span>
            <span style={{ fontSize: 11, color: SOURCE_TONE[f.source] ?? "var(--ink-4)" }}>{f.source}</span>
          </div>
          {canManage && (
            <div style={{ display: "flex", gap: 6 }}>
              <Button variant="outline" size="sm" onClick={() => onSet(f.key, true)} disabled={busy}>On</Button>
              <Button variant="outline" size="sm" onClick={() => onSet(f.key, false)} disabled={busy}>Off</Button>
              <Button variant="ghost" size="sm" onClick={() => onClear(f.key)} disabled={busy || f.source !== clearableSource}>Clear</Button>
            </div>
          )}
        </div>
      ))}
      {rows.length === 0 && <p style={{ color: "var(--ink-3)", fontSize: 13, padding: "12px 18px" }}>No flags.</p>}
    </HospitalityCard>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label style={{ display: "flex", flexDirection: "column", gap: 6 }}>
      <span className="eyebrow">{label}</span>
      {children}
    </label>
  )
}

function SelectBox({ value, onChange, children }: { value: string | number; onChange: (v: string) => void; children: React.ReactNode }) {
  return (
    <select
      value={value}
      onChange={(e) => onChange(e.target.value)}
      style={{ height: 32, borderRadius: "var(--rad-md)", border: "1px solid var(--line-2)", background: "var(--bg-elev-1)", color: "var(--ink-1)", padding: "0 10px", fontSize: 14, fontFamily: "inherit" }}
    >
      {children}
    </select>
  )
}
