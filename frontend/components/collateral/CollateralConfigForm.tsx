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

type SetFn = <K extends keyof CollateralConfig>(key: K, value: CollateralConfig[K]) => void

// Hoisted to module scope so their component identity is stable across parent
// re-renders — defining them inside CollateralConfigForm remounted the inputs on
// every keystroke, which dropped focus after each character.
function TextField({ field, label, placeholder, config, set, disabled }: {
  field: keyof CollateralConfig
  label: string
  placeholder?: string
  config: CollateralConfig
  set: SetFn
  disabled?: boolean
}) {
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

function Toggle({ field, label, hint, config, set, disabled }: {
  field: keyof CollateralConfig
  label: string
  hint?: string
  config: CollateralConfig
  set: SetFn
  disabled?: boolean
}) {
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

export function CollateralConfigForm({
  config,
  onChange,
  disabled,
}: {
  config: CollateralConfig
  onChange: (next: CollateralConfig) => void
  disabled?: boolean
}) {
  const set: SetFn = (key, value) => onChange({ ...config, [key]: value })
  const allows = (b: CollateralBlock) => formatAllows(config.format, b)

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
        {allows("logo") && <Toggle field="showLogo" label="Show logo" hint="when a restaurant logo is set" config={config} set={set} disabled={disabled} />}
        {allows("branch") && <Toggle field="showBranch" label="Show branch" config={config} set={set} disabled={disabled} />}
        {allows("wifi") && <Toggle field="showWifi" label="Show WiFi" config={config} set={set} disabled={disabled} />}
        {allows("footer") && <Toggle field="showFooter" label="Show footer note" config={config} set={set} disabled={disabled} />}
        {!allows("logo") && !allows("branch") && !allows("wifi") && !allows("footer") && (
          <p style={{ fontSize: 12, color: "var(--ink-4)", margin: 0 }}>This format is minimal — no optional blocks.</p>
        )}
      </div>

      {/* Text content — only fields the format supports */}
      <div style={{ display: "flex", flexDirection: "column", gap: 14 }}>
        <div style={labelStyle}>Content</div>
        {allows("welcome") && <TextField field="welcomeMessage" label="Welcome message" placeholder="Scan to begin your evening" config={config} set={set} disabled={disabled} />}
        {allows("subtitle") && <TextField field="subtitle" label="Subtitle" placeholder="No app download required" config={config} set={set} disabled={disabled} />}
        {allows("tagline") && <TextField field="tagline" label="Tagline" placeholder="Fine dining reimagined" config={config} set={set} disabled={disabled} />}
        {allows("branch") && <TextField field="branchDisplay" label="Branch display" placeholder="Bandra" config={config} set={set} disabled={disabled} />}
        {allows("footer") && <TextField field="footerNote" label="Footer note" placeholder="Thank you for dining with us" config={config} set={set} disabled={disabled} />}
        {allows("wifi") && (
          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 10 }}>
            <TextField field="wifiName" label="WiFi name" placeholder="Lounge-Guest" config={config} set={set} disabled={disabled} />
            <TextField field="wifiPassword" label="WiFi password" placeholder="welcome123" config={config} set={set} disabled={disabled} />
          </div>
        )}
        {allows("socials") && (
          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 10 }}>
            <TextField field="instagram" label="Instagram" placeholder="maisonsaffron" config={config} set={set} disabled={disabled} />
            <TextField field="website" label="Website" placeholder="maisonsaffron.com" config={config} set={set} disabled={disabled} />
          </div>
        )}
      </div>
    </div>
  )
}
