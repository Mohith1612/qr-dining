"use client"

import { useEffect, useState } from "react"
import { useStaffStore } from "@/store/staff"
import { staffApi } from "@/lib/api/staff"
import { HospitalityCard } from "@/components/shared/HospitalityCard"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { useBrandingStore } from "@/store/branding"
import { Loader2 } from "lucide-react"
import { toast } from "sonner"

const PRESET_THEMES = [
  {
    id: "dark-luxury",
    name: "Dark Luxury",
    description: "Warm, intimate, evening dining",
    colors: { bg: "#0E0C09", elev: "#1C1914", accent: "#C9A876", ink: "#F4E8D1" },
  },
  {
    id: "modern-minimal",
    name: "Modern Minimal",
    description: "Airy, daytime, casual bistro",
    colors: { bg: "#FAFAF7", elev: "#F0EDE8", accent: "#16140F", ink: "#16140F" },
  },
] as const

type ThemeId = (typeof PRESET_THEMES)[number]["id"]

export function AppearanceTab() {
  const { branchId, token, role } = useStaffStore()
  const [selectedTheme, setSelectedTheme] = useState<ThemeId>("dark-luxury")
  const [logoUrl, setLogoUrl] = useState("")
  const [logoError, setLogoError] = useState(false)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const canEdit = role === "owner" || role === "manager"

  useEffect(() => {
    if (!branchId || !token) { setLoading(false); return }
    staffApi
      .getBranch(branchId, token)
      .then(b => { if (b.theme) setSelectedTheme(b.theme as ThemeId); setLogoUrl(b.logo_url ?? "") })
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [branchId, token])

  async function handleSave() {
    if (!branchId || !token) return
    setSaving(true)
    try {
      await staffApi.updateBranch(branchId, { theme: selectedTheme, logo_url: logoUrl.trim() }, token)
      document.documentElement.dataset.theme = selectedTheme
      // Reflect the new logo in the header immediately.
      useBrandingStore.getState().setBranding({ logoUrl: logoUrl.trim() })
      toast.success("Appearance saved")
    } catch {
      toast.error("Failed to save appearance")
    } finally {
      setSaving(false)
    }
  }

  if (loading) return null

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
      <HospitalityCard elev={1} style={{ padding: "20px 20px" }}>
        <p className="eyebrow" style={{ marginBottom: 6 }}>Theme</p>
        <p className="text-sm" style={{ color: "var(--ink-3)", marginBottom: 18 }}>
          Your guests will see this theme on their phones when they scan the table QR code.
        </p>

        <div style={{ display: "flex", gap: 12, flexWrap: "wrap" }}>
          {PRESET_THEMES.map(theme => {
            const isSelected = selectedTheme === theme.id
            return (
              <button
                key={theme.id}
                onClick={() => canEdit && setSelectedTheme(theme.id)}
                disabled={!canEdit}
                style={{
                  width: 160,
                  background: "none",
                  border: `2px solid ${isSelected ? "var(--accent)" : "var(--line-1)"}`,
                  borderRadius: "var(--rad-md)",
                  padding: 0,
                  cursor: canEdit ? "pointer" : "default",
                  overflow: "hidden",
                  boxShadow: isSelected ? "0 0 0 1px var(--accent)" : "none",
                  transition: "border-color 0.15s, box-shadow 0.15s",
                }}
              >
                {/* Color preview area */}
                <div style={{ background: theme.colors.bg, height: 80, padding: 12, display: "flex", flexDirection: "column", gap: 6 }}>
                  <div style={{ display: "flex", gap: 5 }}>
                    <div style={{ width: 28, height: 14, borderRadius: 4, background: theme.colors.elev }} />
                    <div style={{ width: 28, height: 14, borderRadius: 4, background: theme.colors.accent }} />
                    <div style={{ width: 28, height: 14, borderRadius: 4, background: theme.colors.ink, opacity: 0.7 }} />
                  </div>
                  <div style={{ width: "100%", height: 6, borderRadius: 3, background: theme.colors.elev }} />
                  <div style={{ width: "70%", height: 6, borderRadius: 3, background: theme.colors.elev }} />
                </div>
                {/* Label */}
                <div style={{ background: theme.colors.elev, padding: "8px 10px", textAlign: "left" }}>
                  <p style={{ margin: 0, fontSize: 12, fontWeight: 600, color: theme.colors.ink, lineHeight: 1.3 }}>
                    {theme.name}
                  </p>
                  {isSelected && (
                    <p style={{ margin: "2px 0 0", fontSize: 11, color: theme.colors.accent, lineHeight: 1.2 }}>
                      Selected
                    </p>
                  )}
                </div>
              </button>
            )
          })}
        </div>

      </HospitalityCard>

      {/* Logo */}
      <HospitalityCard elev={1} style={{ padding: "20px 20px" }}>
        <p className="eyebrow" style={{ marginBottom: 6 }}>Logo</p>
        <p className="text-sm" style={{ color: "var(--ink-3)", marginBottom: 14 }}>
          Shown in the header of your guest and staff apps. Paste an image URL (PNG/SVG/JPG),
          ideally a transparent PNG. A hosted upload option appears here once storage is configured.
        </p>
        <div style={{ display: "flex", gap: 14, alignItems: "center", flexWrap: "wrap" }}>
          <div style={{
            width: 56, height: 56, borderRadius: "var(--rad-md)", flexShrink: 0,
            border: "1px solid var(--line-2)", background: "var(--bg-elev-2)",
            display: "flex", alignItems: "center", justifyContent: "center", overflow: "hidden",
          }}>
            {(logoUrl.trim() && !logoError) ? (
              // eslint-disable-next-line @next/next/no-img-element
              <img src={logoUrl.trim()} alt="Logo preview" onError={() => setLogoError(true)} style={{ width: "100%", height: "100%", objectFit: "contain" }} />
            ) : (
              <span style={{ fontSize: 11, color: "var(--ink-4)" }}>none</span>
            )}
          </div>
          <Input
            value={logoUrl}
            onChange={(e) => { setLogoUrl(e.target.value); setLogoError(false) }}
            placeholder="https://…/logo.png"
            disabled={!canEdit}
            style={{ flex: 1, minWidth: 220 }}
          />
        </div>
        {logoUrl.trim() && (
          <button
            onClick={() => { setLogoUrl(""); setLogoError(false) }}
            disabled={!canEdit}
            style={{ marginTop: 10, fontSize: 12.5, color: "var(--ink-3)", background: "none", border: "none", cursor: canEdit ? "pointer" : "default", padding: 0 }}
          >
            Remove logo
          </button>
        )}
      </HospitalityCard>

      {canEdit && (
        <div>
          <Button onClick={handleSave} disabled={saving}>
            {saving ? <Loader2 className="size-4 animate-spin" /> : "Save appearance"}
          </Button>
        </div>
      )}
    </div>
  )
}

