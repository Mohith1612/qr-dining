"use client"

import { useState } from "react"
import { useRouter } from "next/navigation"
import { usePlatformStore } from "@/store/platform"
import { platformApi } from "@/lib/api/platform"
import { isMFAChallenge, type PlatformSession } from "@/types/platform"
import { HospitalityCard } from "@/components/shared/HospitalityCard"
import { Loader2, ShieldCheck } from "lucide-react"
import { toast } from "sonner"

const fieldStyle: React.CSSProperties = {
  display: "block", width: "100%", fontSize: 16, boxSizing: "border-box",
  background: "transparent", outline: "none", border: "none",
  borderBottom: "1px solid var(--line-2)", paddingBottom: 10,
  color: "var(--ink-1)", fontFamily: "inherit",
}

export default function PlatformLoginPage() {
  const router = useRouter()
  const setSession = usePlatformStore((s) => s.setSession)

  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [loading, setLoading] = useState(false)

  // MFA second step
  const [mfaChallenge, setMfaChallenge] = useState<string | null>(null)
  const [mfaCode, setMfaCode] = useState("")

  function commitSession(s: PlatformSession) {
    setSession({
      token: s.token,
      platformUserId: s.platform_user_id,
      email: s.email,
      displayName: s.display_name,
      roles: s.roles,
      expiresAt: s.expires_at,
    })
    router.replace("/platform")
  }

  async function handleLogin(e: React.FormEvent) {
    e.preventDefault()
    if (!email.trim() || !password) return
    setLoading(true)
    try {
      const res = await platformApi.auth(email.trim(), password)
      if (isMFAChallenge(res)) {
        setMfaChallenge(res.mfa_challenge)
        toast.info("Enter your authenticator code to continue.")
      } else {
        commitSession(res)
      }
    } catch {
      toast.error("Invalid credentials.")
    } finally {
      setLoading(false)
    }
  }

  async function handleMFA(e: React.FormEvent) {
    e.preventDefault()
    if (!mfaChallenge || !mfaCode.trim()) return
    setLoading(true)
    try {
      const s = await platformApi.completeMFA(mfaChallenge, mfaCode.trim())
      commitSession(s)
    } catch {
      toast.error("Invalid or expired code.")
    } finally {
      setLoading(false)
    }
  }

  return (
    <div
      data-surface="platform"
      className="atmos min-h-screen flex items-center justify-center px-6"
      style={{ background: "var(--bg-base)", color: "var(--ink-1)", backgroundImage: "var(--glow-warm)" }}
    >
      <div style={{ width: "100%", maxWidth: 420 }}>
        <div style={{ display: "flex", flexDirection: "column", alignItems: "center", gap: 16, marginBottom: 32 }}>
          <div style={{
            width: 64, height: 64,
            background: "linear-gradient(160deg, var(--bg-elev-2), var(--bg-elev-1))",
            border: "2px solid var(--line-2)", borderRadius: "var(--rad-xl)",
            boxShadow: "var(--shadow-3)",
            display: "flex", alignItems: "center", justifyContent: "center",
          }}>
            <ShieldCheck size={28} style={{ color: "var(--accent)" }} aria-hidden />
          </div>
          <div style={{ textAlign: "center", display: "flex", flexDirection: "column", gap: 4 }}>
            <p className="eyebrow">Platform · Control Plane</p>
            <h1 className="serif" style={{ fontSize: 30, fontWeight: 500, margin: 0 }}>
              {mfaChallenge ? "Verify identity" : "Operator sign in"}
            </h1>
            <p style={{ fontSize: 13, color: "var(--ink-3)", margin: 0 }}>
              {mfaChallenge ? "Enter your authenticator or recovery code" : "Platform operators only"}
            </p>
          </div>
        </div>

        <HospitalityCard elev={3} style={{ padding: "28px 24px" }}>
          {!mfaChallenge ? (
            <form onSubmit={handleLogin}>
              <p className="eyebrow" style={{ marginBottom: 8 }}>Email</p>
              <input
                type="email" autoComplete="username" placeholder="ops@example.com"
                value={email} onChange={(e) => setEmail(e.target.value)} required style={fieldStyle}
              />
              <div style={{ height: 20 }} />
              <p className="eyebrow" style={{ marginBottom: 8 }}>Password</p>
              <input
                type="password" autoComplete="current-password" placeholder="••••••••"
                value={password} onChange={(e) => setPassword(e.target.value)} required style={fieldStyle}
              />
              <div style={{ height: 24 }} />
              <SubmitButton loading={loading} disabled={!email || !password} label="Sign in" />
            </form>
          ) : (
            <form onSubmit={handleMFA}>
              <p className="eyebrow" style={{ marginBottom: 8 }}>Authentication code</p>
              <input
                type="text" inputMode="numeric" autoComplete="one-time-code" placeholder="123456"
                value={mfaCode} onChange={(e) => setMfaCode(e.target.value)} required autoFocus style={fieldStyle}
              />
              <div style={{ height: 24 }} />
              <SubmitButton loading={loading} disabled={!mfaCode} label="Verify" />
              <button
                type="button"
                onClick={() => { setMfaChallenge(null); setMfaCode("") }}
                style={{ marginTop: 14, width: "100%", background: "none", border: "none", color: "var(--ink-3)", fontSize: 12, cursor: "pointer" }}
              >
                Back to sign in
              </button>
            </form>
          )}
        </HospitalityCard>
      </div>
    </div>
  )
}

function SubmitButton({ loading, disabled, label }: { loading: boolean; disabled: boolean; label: string }) {
  const blocked = loading || disabled
  return (
    <button
      type="submit"
      disabled={blocked}
      className="press"
      style={{
        display: "flex", alignItems: "center", justifyContent: "center", gap: 8,
        width: "100%", height: 48,
        background: "var(--accent)", color: "var(--accent-ink)",
        border: "none", borderRadius: "var(--rad-md)", fontSize: 15, fontWeight: 600,
        cursor: blocked ? "not-allowed" : "pointer", opacity: blocked ? 0.55 : 1,
      }}
    >
      {loading ? <><Loader2 className="animate-spin" style={{ width: 16, height: 16 }} /> Please wait…</> : label}
    </button>
  )
}
