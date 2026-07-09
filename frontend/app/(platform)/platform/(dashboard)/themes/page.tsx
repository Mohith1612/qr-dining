"use client"

import { useCallback, useEffect, useState } from "react"
import { usePlatformStore } from "@/store/platform"
import { platformApi } from "@/lib/api/platform"
import { hasPlatformRole } from "@/lib/platform-rbac"
import { ApiError } from "@/lib/api/client"
import type { Organization, ThemePreset } from "@/types/platform"
import { HospitalityCard } from "@/components/shared/HospitalityCard"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { PageHeader, Section, PlatformLoading } from "@/components/platform/ui"
import { Check, Loader2, Lock } from "lucide-react"
import { toast } from "sonner"

const HEX_RE = /^#[0-9a-fA-F]{6}([0-9a-fA-F]{2})?$/

export default function ThemesPage() {
  const { token, roles } = usePlatformStore()
  const canManage = hasPlatformRole(roles)

  const [orgs, setOrgs] = useState<Organization[]>([])
  const [presets, setPresets] = useState<ThemePreset[]>([])
  const [allowedKeys, setAllowedKeys] = useState<string[]>([])
  const [orgId, setOrgId] = useState<number | undefined>(undefined)

  const [preset, setPreset] = useState<string>("")
  const [tokens, setTokens] = useState<Record<string, string>>({})
  const [customAllowed, setCustomAllowed] = useState(false)
  const [loading, setLoading] = useState(true)
  const [loadingOrg, setLoadingOrg] = useState(false)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!token) return
    Promise.all([
      platformApi.listOrganizations(token),
      platformApi.listThemePresets(token),
    ]).then(([o, p]) => {
      setOrgs(o.organizations)
      setPresets(p.presets)
      setAllowedKeys(p.allowed_token_keys)
      setOrgId(o.organizations[0]?.id)
    }).catch(() => toast.error("Couldn't load theme data."))
      .finally(() => setLoading(false))
  }, [token])

  const loadOrgTheme = useCallback(async () => {
    if (!token || !orgId) return
    setLoadingOrg(true)
    try {
      const [{ theme }, { entitlements }] = await Promise.all([
        platformApi.getOrganizationTheme(orgId, token),
        platformApi.getOrganizationEntitlements(orgId, token),
      ])
      setPreset(theme.preset)
      setTokens(theme.tokens ?? {})
      setCustomAllowed(Boolean(entitlements.capabilities["custom.theme"]))
    } catch {
      toast.error("Couldn't load organization theme.")
    } finally {
      setLoadingOrg(false)
    }
  }, [token, orgId])

  useEffect(() => { loadOrgTheme() }, [loadOrgTheme])

  function setTokenValue(key: string, value: string) {
    setTokens((t) => {
      const next = { ...t }
      if (value === "") delete next[key]
      else next[key] = value
      return next
    })
  }

  function invalidTokens(): string[] {
    return Object.entries(tokens).filter(([, v]) => !HEX_RE.test(v)).map(([k]) => k)
  }

  async function save() {
    if (!token || !orgId) return
    const bad = invalidTokens()
    if (bad.length > 0) {
      toast.error(`Invalid hex for: ${bad.join(", ")}`)
      return
    }
    setSaving(true)
    try {
      // Only send custom tokens when entitled; otherwise preset-only.
      const payload = customAllowed ? tokens : {}
      const { theme } = await platformApi.setOrganizationTheme(orgId, preset, payload, token)
      setPreset(theme.preset)
      setTokens(theme.tokens ?? {})
      toast.success("Theme saved.")
    } catch (e) {
      if (e instanceof ApiError && e.code === "FORBIDDEN") toast.error("Custom theme requires the custom.theme entitlement.")
      else if (e instanceof ApiError && e.code === "VALIDATION_ERROR") toast.error(e.message)
      else if (e instanceof ApiError && e.code === "THEME_PRESET_NOT_FOUND") toast.error("Unknown preset.")
      else toast.error("Couldn't save theme.")
    } finally {
      setSaving(false)
    }
  }

  if (loading) return <PlatformLoading />

  return (
    <div>
      <PageHeader title="Themes" subtitle="Branding presets and validated design tokens (no arbitrary CSS)" />

      <label style={{ display: "flex", flexDirection: "column", gap: 6, maxWidth: 360, marginBottom: 20 }}>
        <span className="eyebrow">Organization</span>
        <select
          value={orgId ?? ""}
          onChange={(e) => setOrgId(e.target.value ? Number(e.target.value) : undefined)}
          style={{ height: 32, borderRadius: "var(--rad-md)", border: "1px solid var(--line-2)", background: "var(--bg-elev-1)", color: "var(--ink-1)", padding: "0 10px", fontSize: 14, fontFamily: "inherit" }}
        >
          {orgs.map((o) => <option key={o.id} value={o.id}>{o.name} ({o.code})</option>)}
        </select>
      </label>

      {loadingOrg ? <PlatformLoading /> : (
        <>
          <Section title="Preset" actions={canManage ? <Button onClick={save} disabled={saving}>{saving ? <Loader2 className="animate-spin" style={{ width: 14, height: 14 }} /> : null} Save theme</Button> : undefined}>
            <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(180px, 1fr))", gap: 10 }}>
              {presets.map((p) => {
                const active = preset === p.key
                return (
                  <HospitalityCard
                    key={p.key}
                    elev={active ? 2 : 1}
                    press={canManage}
                    onClick={canManage ? () => setPreset(p.key) : undefined}
                    style={{ padding: "14px 16px", border: active ? "1px solid var(--accent)" : undefined, display: "flex", flexDirection: "column", gap: 4 }}
                  >
                    <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
                      <span style={{ fontSize: 14, fontWeight: 600, color: "var(--ink-1)" }}>{p.name}</span>
                      {active && <Check size={15} style={{ color: "var(--accent)" }} aria-hidden />}
                    </div>
                    <span className="mono" style={{ fontSize: 11, color: "var(--ink-4)" }}>{p.key}</span>
                    <span style={{ fontSize: 12, color: "var(--ink-3)" }}>{p.description}</span>
                  </HospitalityCard>
                )
              })}
            </div>
          </Section>

          <Section title="Custom tokens">
            {!customAllowed ? (
              <HospitalityCard elev={1} style={{ padding: "20px", display: "flex", alignItems: "center", gap: 12 }}>
                <Lock size={18} style={{ color: "var(--ink-4)" }} aria-hidden />
                <div>
                  <p style={{ fontSize: 14, color: "var(--ink-1)", margin: 0 }}>Custom tokens require the <span className="mono">custom.theme</span> entitlement.</p>
                  <p style={{ fontSize: 13, color: "var(--ink-3)", margin: "4px 0 0" }}>Grant it from this organization&apos;s entitlements to enable token overrides.</p>
                </div>
              </HospitalityCard>
            ) : (
              <HospitalityCard elev={1} style={{ padding: "8px 0" }}>
                {allowedKeys.map((key) => {
                  const val = tokens[key] ?? ""
                  const valid = val === "" || HEX_RE.test(val)
                  return (
                    <div key={key} style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, padding: "8px 18px", borderBottom: "1px solid var(--line-1)" }}>
                      <span className="mono" style={{ fontSize: 13, color: "var(--ink-1)" }}>{key}</span>
                      <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                        <span style={{ width: 20, height: 20, borderRadius: 4, border: "1px solid var(--line-2)", background: valid && val ? val : "transparent" }} aria-hidden />
                        <Input
                          value={val}
                          disabled={!canManage}
                          onChange={(e) => setTokenValue(key, e.target.value)}
                          placeholder="#RRGGBB"
                          aria-invalid={!valid}
                          style={{ width: 130, fontFamily: "var(--font-mono, monospace)", borderColor: valid ? undefined : "var(--alert)" }}
                        />
                      </div>
                    </div>
                  )
                })}
              </HospitalityCard>
            )}
            <p style={{ color: "var(--ink-4)", fontSize: 12, marginTop: 8 }}>
              Values must be hex colors (<span className="mono">#RRGGBB</span> or <span className="mono">#RRGGBBAA</span>). Blank = inherit preset.
            </p>
          </Section>
        </>
      )}
    </div>
  )
}
