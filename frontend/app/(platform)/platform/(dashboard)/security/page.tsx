"use client"

import dynamic from "next/dynamic"
import { useState } from "react"
import { usePlatformStore } from "@/store/platform"
import { platformApi } from "@/lib/api/platform"
import { ApiError } from "@/lib/api/client"
import { HospitalityCard } from "@/components/shared/HospitalityCard"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { PageHeader, Section } from "@/components/platform/ui"
import { Loader2, ShieldCheck, KeyRound, Copy, Check } from "lucide-react"
import { toast } from "sonner"

const QRCodeSVG = dynamic(() => import("qrcode.react").then((m) => m.QRCodeSVG), { ssr: false })

type Stage = "idle" | "enrolling" | "confirmed"

export default function SecurityPage() {
  const { token, email } = usePlatformStore()

  const [stage, setStage] = useState<Stage>("idle")
  const [setup, setSetup] = useState<{ secret: string; otpauth_uri: string } | null>(null)
  const [code, setCode] = useState("")
  const [recoveryCodes, setRecoveryCodes] = useState<string[]>([])
  const [busy, setBusy] = useState(false)
  const [copied, setCopied] = useState(false)

  const [disableCode, setDisableCode] = useState("")
  const [disabling, setDisabling] = useState(false)

  async function beginEnroll() {
    if (!token) return
    setBusy(true)
    try {
      const s = await platformApi.enrollMfa(token)
      setSetup(s)
      setStage("enrolling")
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Couldn't start enrollment.")
    } finally {
      setBusy(false)
    }
  }

  async function confirmEnroll(e: React.FormEvent) {
    e.preventDefault()
    if (!token || code.trim().length < 6) return
    setBusy(true)
    try {
      const { recovery_codes } = await platformApi.confirmMfa(code.trim(), token)
      setRecoveryCodes(recovery_codes)
      setStage("confirmed")
      setCode("")
      toast.success("Two-factor authentication enabled.")
    } catch (err) {
      toast.error(err instanceof ApiError && err.code === "MFA_INVALID_CODE"
        ? "That code didn't match. Check your authenticator and try again."
        : err instanceof ApiError ? err.message : "Couldn't confirm the code.")
    } finally {
      setBusy(false)
    }
  }

  async function disable(e: React.FormEvent) {
    e.preventDefault()
    if (!token || disableCode.trim().length < 6) return
    setDisabling(true)
    try {
      await platformApi.disableMfa(disableCode.trim(), token)
      setDisableCode("")
      setStage("idle")
      setSetup(null)
      setRecoveryCodes([])
      toast.success("Two-factor authentication disabled.")
    } catch (err) {
      toast.error(err instanceof ApiError && err.code === "MFA_INVALID_CODE"
        ? "That code didn't match. Enter a current code or a recovery code."
        : err instanceof ApiError ? err.message : "Couldn't disable MFA.")
    } finally {
      setDisabling(false)
    }
  }

  function copyRecovery() {
    navigator.clipboard?.writeText(recoveryCodes.join("\n")).then(
      () => { setCopied(true); setTimeout(() => setCopied(false), 2000) },
      () => toast.error("Couldn't copy — select the codes manually."),
    )
  }

  return (
    <div>
      <PageHeader
        title="Security"
        subtitle={`Two-factor authentication for ${email ?? "your platform account"}`}
      />

      <Section title="Authenticator app">
        <HospitalityCard elev={2} style={{ padding: 22 }}>
          {stage === "idle" && (
            <div style={{ display: "flex", flexDirection: "column", gap: 14, maxWidth: 460 }}>
              <div style={{ display: "flex", gap: 12, alignItems: "flex-start" }}>
                <ShieldCheck size={22} style={{ color: "var(--accent)", flexShrink: 0, marginTop: 2 }} aria-hidden />
                <p style={{ margin: 0, color: "var(--ink-2)", fontSize: 14, lineHeight: 1.55 }}>
                  Protect this platform account with a time-based one-time code from an authenticator
                  app (Google Authenticator, 1Password, Authy). You&apos;ll enter a code at each login.
                </p>
              </div>
              <div>
                <Button onClick={beginEnroll} disabled={busy}>
                  {busy ? <Loader2 className="animate-spin" style={{ width: 14, height: 14 }} /> : <KeyRound size={15} />}
                  Set up authenticator
                </Button>
              </div>
            </div>
          )}

          {stage === "enrolling" && setup && (
            <form onSubmit={confirmEnroll} style={{ display: "flex", flexDirection: "column", gap: 18, maxWidth: 460 }}>
              <p style={{ margin: 0, color: "var(--ink-2)", fontSize: 14, lineHeight: 1.55 }}>
                Scan this with your authenticator app, then enter the 6-digit code it shows.
              </p>
              <div style={{ display: "flex", gap: 20, alignItems: "center", flexWrap: "wrap" }}>
                <div style={{ background: "#fff", padding: 12, borderRadius: 12, lineHeight: 0 }}>
                  <QRCodeSVG value={setup.otpauth_uri} size={148} level="M" />
                </div>
                <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
                  <span className="eyebrow" style={{ fontSize: 10 }}>Or enter this key manually</span>
                  <code style={{
                    fontFamily: "var(--font-mono, ui-monospace), monospace", fontSize: 13,
                    color: "var(--ink-1)", background: "var(--bg-elev-2)", padding: "8px 10px",
                    borderRadius: 8, border: "1px solid var(--line-2)", wordBreak: "break-all", maxWidth: 220,
                  }}>{setup.secret}</code>
                </div>
              </div>
              <div style={{ display: "flex", gap: 10, alignItems: "flex-end", flexWrap: "wrap" }}>
                <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
                  <label htmlFor="mfa-code" className="eyebrow" style={{ fontSize: 10 }}>6-digit code</label>
                  <Input
                    id="mfa-code"
                    inputMode="numeric"
                    autoComplete="one-time-code"
                    placeholder="000000"
                    value={code}
                    onChange={(e) => setCode(e.target.value.replace(/\D/g, "").slice(0, 6))}
                    style={{ width: 130, letterSpacing: "0.3em", fontVariantNumeric: "tabular-nums" }}
                  />
                </div>
                <Button type="submit" disabled={busy || code.length < 6}>
                  {busy ? <Loader2 className="animate-spin" style={{ width: 14, height: 14 }} /> : <Check size={15} />}
                  Verify &amp; enable
                </Button>
                <Button type="button" variant="ghost" onClick={() => { setStage("idle"); setSetup(null); setCode("") }} disabled={busy}>
                  Cancel
                </Button>
              </div>
            </form>
          )}

          {stage === "confirmed" && (
            <div style={{ display: "flex", flexDirection: "column", gap: 14, maxWidth: 460 }}>
              <div style={{ display: "flex", gap: 10, alignItems: "center", color: "var(--ok)" }}>
                <Check size={18} aria-hidden />
                <strong style={{ fontSize: 15 }}>Two-factor authentication is on.</strong>
              </div>
              <p style={{ margin: 0, color: "var(--ink-2)", fontSize: 14, lineHeight: 1.55 }}>
                Save these recovery codes somewhere safe. Each works once if you lose your
                authenticator — <strong>they won&apos;t be shown again.</strong>
              </p>
              <div style={{
                display: "grid", gridTemplateColumns: "repeat(2, minmax(0,1fr))", gap: 8,
                background: "var(--bg-elev-2)", border: "1px solid var(--line-2)", borderRadius: 10, padding: 14,
              }}>
                {recoveryCodes.map((rc) => (
                  <code key={rc} style={{ fontFamily: "var(--font-mono, ui-monospace), monospace", fontSize: 13.5, color: "var(--ink-1)", letterSpacing: "0.04em" }}>{rc}</code>
                ))}
              </div>
              <div>
                <Button variant="secondary" onClick={copyRecovery}>
                  {copied ? <Check size={15} /> : <Copy size={15} />}
                  {copied ? "Copied" : "Copy codes"}
                </Button>
              </div>
            </div>
          )}
        </HospitalityCard>
      </Section>

      <Section title="Disable">
        <HospitalityCard elev={2} style={{ padding: 22 }}>
          <form onSubmit={disable} style={{ display: "flex", flexDirection: "column", gap: 12, maxWidth: 460 }}>
            <p style={{ margin: 0, color: "var(--ink-2)", fontSize: 14, lineHeight: 1.55 }}>
              Turn off two-factor authentication. Enter a current authenticator code or a recovery code to confirm.
            </p>
            <div style={{ display: "flex", gap: 10, alignItems: "flex-end", flexWrap: "wrap" }}>
              <Input
                inputMode="text"
                autoComplete="one-time-code"
                placeholder="Code"
                value={disableCode}
                onChange={(e) => setDisableCode(e.target.value.trim())}
                style={{ width: 160 }}
                aria-label="Current code or recovery code"
              />
              <Button type="submit" variant="destructive" disabled={disabling || disableCode.length < 6}>
                {disabling ? <Loader2 className="animate-spin" style={{ width: 14, height: 14 }} /> : null}
                Disable 2FA
              </Button>
            </div>
          </form>
        </HospitalityCard>
      </Section>
    </div>
  )
}
