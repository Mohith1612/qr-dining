"use client"

import { useState } from "react"
import { useSession } from "@/hooks/useSession"
import { useAssistance } from "@/hooks/useAssistance"
import { AssistSkeleton } from "@/components/shared/LoadingSkeleton"
import { StatusBadge } from "@/components/shared/StatusBadge"
import { relativeTime } from "@/lib/format"
import { Bell, CreditCard, MessageSquare, CheckCircle, Loader2, ChevronRight } from "lucide-react"
import { toast } from "sonner"
import { ApiError, friendlyErrorMessage } from "@/lib/api/client"
import type { AssistanceRequest, AssistanceType } from "@/types/api"

const ASSIST_OPTIONS: {
  type: AssistanceType
  label: string
  description: string
  icon: React.ElementType
}[] = [
  {
    type: "waiter",
    label: "Call Waiter",
    description: "Need help with your order or something at the table",
    icon: Bell,
  },
  {
    type: "bill",
    label: "Request Bill",
    description: "Ready to pay? We'll bring the bill right over",
    icon: CreditCard,
  },
  {
    type: "other",
    label: "Other",
    description: "Anything else — we're happy to help",
    icon: MessageSquare,
  },
]

const TYPE_LABEL: Record<AssistanceType, string> = {
  waiter: "Waiter",
  bill: "Bill",
  other: "Other",
}

function ActiveRequestCard({ request }: { request: AssistanceRequest }) {
  return (
    <div style={{
      background: "linear-gradient(180deg, var(--bg-elev-2), var(--bg-elev-1))",
      border: "1px solid var(--line-2)", borderRadius: "var(--rad-lg)",
      boxShadow: "var(--shadow-2)", padding: 16, marginBottom: 12,
    }}>
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 8, marginBottom: 8 }}>
        <div>
          <p className="serif" style={{ fontSize: 16, fontWeight: 600, color: "var(--ink-1)", lineHeight: 1.2 }}>
            {TYPE_LABEL[request.type]} request
          </p>
          <p style={{ fontSize: 12, color: "var(--ink-3)", marginTop: 2 }}>{relativeTime(request.created_at)}</p>
        </div>
        <StatusBadge status={request.status} />
      </div>
      <div style={{ display: "flex", alignItems: "center", gap: 6, color: "var(--ink-2)", fontSize: 12.5, marginTop: 8 }}>
        <span className="live-dot" />
        {request.status === "pending" ? "A staff member will be with you shortly." : "Someone is on their way."}
      </div>
    </div>
  )
}

export default function AssistPage() {
  const { session } = useSession()
  const { active, requestAssistance } = useAssistance()
  const [requesting, setRequesting] = useState<AssistanceType | null>(null)

  if (!session) return <AssistSkeleton />

  const activeTypes = new Set(active.map((r) => r.type))

  async function handleRequest(type: AssistanceType) {
    if (activeTypes.has(type)) return
    setRequesting(type)
    try {
      await requestAssistance(type)
      toast.success("Request sent — we'll be right with you.")
    } catch (err) {
      toast.error(err instanceof ApiError ? friendlyErrorMessage(err.code) : "Couldn't send request. Please try again.")
    } finally {
      setRequesting(null)
    }
  }

  return (
    <div className="screen-enter" style={{ background: "var(--bg-base)", color: "var(--ink-1)" }}>
      {/* Header */}
      <div className="page-glow" style={{ padding: "24px 20px 16px", marginBottom: 4 }}>
        <p className="eyebrow">At your service</p>
        <h1 className="display-lg" style={{ margin: "6px 0 4px" }}>
          How can we help?
        </h1>
        <p style={{ margin: 0, color: "var(--ink-2)", fontSize: 13.5, lineHeight: 1.5 }}>Tap below and a team member will come to you.</p>
      </div>
      <div style={{ padding: "0 20px 28px" }}>

      {active.length > 0 && (
        <div style={{ marginBottom: 24 }}>
          <p className="eyebrow" style={{ marginBottom: 10 }}>Active requests</p>
          {active.map((req) => <ActiveRequestCard key={req.id} request={req} />)}
        </div>
      )}

      <div>
        {active.length > 0 && <p className="eyebrow" style={{ marginBottom: 10 }}>Request another</p>}
        {ASSIST_OPTIONS.map(({ type, label, description, icon: Icon }) => {
          const isActive = activeTypes.has(type)
          const isLoading = requesting === type

          return (
            <button
              key={type}
              onClick={() => handleRequest(type)}
              disabled={isActive || requesting !== null}
              className="press"
              style={{
                width: "100%", display: "flex", alignItems: "center", gap: 14,
                textAlign: "left", cursor: isActive ? "default" : "pointer",
                background: "var(--bg-elev-1)",
                border: `1px solid ${isActive ? "var(--line-1)" : "var(--line-2)"}`,
                boxShadow: isActive ? "none" : "var(--shadow-1)",
                borderRadius: "var(--rad-lg)",
                opacity: isActive ? 0.55 : 1,
                padding: "16px",
                marginBottom: 10,
              }}
              aria-label={`${label}: ${description}`}
            >
              <div style={{
                width: 44, height: 44, borderRadius: "var(--rad-md)", flexShrink: 0,
                background: isActive ? "var(--ok-soft)" : "var(--accent-soft)",
                color: isActive ? "var(--ok)" : "var(--accent)",
                display: "flex", alignItems: "center", justifyContent: "center",
              }}>
                {isLoading ? (
                  <Loader2 size={18} className="animate-spin" aria-hidden />
                ) : isActive ? (
                  <CheckCircle size={18} aria-hidden />
                ) : (
                  <Icon size={18} aria-hidden />
                )}
              </div>
              <div style={{ flex: 1 }}>
                <p className="serif" style={{ fontSize: 16, fontWeight: 600, color: "var(--ink-1)", lineHeight: 1.2, marginBottom: 2 }}>{label}</p>
                <p style={{ fontSize: 13, color: "var(--ink-3)", lineHeight: 1.5 }}>{isActive ? "Request already sent" : description}</p>
              </div>
              {!isActive && <ChevronRight size={16} style={{ color: "var(--ink-3)", flexShrink: 0 }} aria-hidden />}
            </button>
          )
        })}
      </div>
      </div>
    </div>
  )
}
