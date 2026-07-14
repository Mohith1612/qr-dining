"use client"

import type { CollateralConfig, CollateralFormat } from "@/types/collateral"
import { ALL_FORMATS, formatAllows, type CollateralBlock } from "@/lib/collateral/formats"

// Structured configuration form — format switcher, content toggles, and text fields.
// No raw HTML/CSS inputs and no drag-and-drop: every field maps to a typed config key.
// Fields/toggles auto-hide when the selected format doesn't support that content block.

const FIELD_LIMITS: Record<string, number> = {
  welcomeMessage: 120, subtitle: 160, footerNote: 160, branchDisplay: 60,
  tagline: 120, wifiName: 64, wifiPassword: 64, instagram: 120, website: 200,
}

const labelStyle: React.CSSProperties = { fontSize: 12, color: "var(--ink-3)", letterSpacing: "0.02em" }
const inputStyle: React.CSSProperties = {
  height: 32, borderRadius: "var(--rad-md)", border: "1px solid var(--line-2)",
  background: "var(--bg-elev-1)", color: "var(--ink-1)", padding: "0 10px", fontSize: 14, fontFamily: "inherit", width: "100%",
}

export function CollateralConfigForm({
  config,
  onChange,
  disabled,
}: {
  config: CollateralConfig
  onChange: (next: CollateralConfig) => void
  disabled?: boolean
}) {
  const set = <K extends keyof CollateralConfig>(key: K, value: CollateralConfig[K]) => onChange({ ...config, [key]: value })
  const allows = (b: CollateralBlock) => formatAllows(config.format, b)

  function TextField({ field, label, placeholder }: { field: keyof CollateralConfig; label: string; placeholder?: string }) {
    return (
      <label style={{ display: "flex", flexDirection: "column", gap: 5 }}>
        <span style={labelStyle}>{label}</span>
        <input
          type="text"
          value={String(config[field] ?? "")}
          maxLength={FIELD_LIMITS[field as string]}
          disabled={disabled}
          placeholder={placeholder}
          onChange={(e) => set(field, e.target.value as CollateralConfig[typeof field])}
          style={inputStyle}
        />
      </label>
    )
  }

  function Toggle({ field, label, hint }: { field: keyof CollateralConfig; label: string; hint?: string }) {
    return (
      <label style={{ display: "flex", alignItems: "center", gap: 10, cursor: disabled ? "default" : "pointer", padding: "6px 0" }}>
        <input
          type="checkbox"
          checked={Boolean(config[field])}
          disabled={disabled}
          onChange={(e) => set(field, e.target.checked as CollateralConfig[typeof field])}
          style={{ width: 16, height: 16, accentColor: "var(--accent)" }}
        />
        <span>
          <span style={{ fontSize: 14, color: "var(--ink-1)" }}>{label}</span>
          {hint ? <span style={{ fontSize: 12, color: "var(--ink-4)", marginLeft: 8 }}>{hint}</span> : null}
        </span>
      </label>
    )
  }

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 22 }}>
      {/* Format */}
      <div>
        <div style={{ ...labelStyle, marginBottom: 8 }}>Print format</div>
        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 8 }}>
          {ALL_FORMATS.map((f) => {
            const active = config.format === f.key
            return (
              <button
                key={f.key}
                type="button"
                disabled={disabled}
                onClick={() => set("format", f.key as CollateralFormat)}
                className="press"
                style={{
                  textAlign: "left",
                  padding: "10px 12px",
                  borderRadius: "var(--rad-md)",
                  border: active ? "1px solid var(--accent)" : "1px solid var(--line-2)",
                  background: active ? "var(--bg-elev-2)" : "var(--bg-elev-1)",
                  color: "var(--ink-1)",
                  cursor: disabled ? "default" : "pointer",
                }}
              >
                <div style={{ fontSize: 13, fontWeight: 600 }}>{f.label}</div>
                <div style={{ fontSize: 11, color: "var(--ink-3)", marginTop: 2, lineHeight: 1.3 }}>{f.description}</div>
              </button>
            )
          })}
        </div>
      </div>

      {/* Content toggles — only those the format supports */}
      <div>
        <div style={{ ...labelStyle, marginBottom: 4 }}>Content blocks</div>
        {allows("logo") && <Toggle field="showLogo" label="Show logo" hint="when a restaurant logo is set" />}
        {allows("branch") && <Toggle field="showBranch" label="Show branch" />}
        {allows("wifi") && <Toggle field="showWifi" label="Show WiFi" />}
        {allows("footer") && <Toggle field="showFooter" label="Show footer note" />}
        {!allows("logo") && !allows("branch") && !allows("wifi") && !allows("footer") && (
          <p style={{ fontSize: 12, color: "var(--ink-4)", margin: 0 }}>This format is minimal — no optional blocks.</p>
        )}
      </div>

      {/* Text content — only fields the format supports */}
      <div style={{ display: "flex", flexDirection: "column", gap: 14 }}>
        <div style={labelStyle}>Content</div>
        {allows("welcome") && <TextField field="welcomeMessage" label="Welcome message" placeholder="Scan to begin your evening" />}
        {allows("subtitle") && <TextField field="subtitle" label="Subtitle" placeholder="No app download required" />}
        {allows("tagline") && <TextField field="tagline" label="Tagline" placeholder="Fine dining reimagined" />}
        {allows("branch") && <TextField field="branchDisplay" label="Branch display" placeholder="Bandra" />}
        {allows("footer") && <TextField field="footerNote" label="Footer note" placeholder="Thank you for dining with us" />}
        {allows("wifi") && (
          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 10 }}>
            <TextField field="wifiName" label="WiFi name" placeholder="Lounge-Guest" />
            <TextField field="wifiPassword" label="WiFi password" placeholder="welcome123" />
          </div>
        )}
        {allows("socials") && (
          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 10 }}>
            <TextField field="instagram" label="Instagram" placeholder="maisonsaffron" />
            <TextField field="website" label="Website" placeholder="maisonsaffron.com" />
          </div>
        )}
      </div>
    </div>
  )
}
