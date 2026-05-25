"use client"

import { useState } from "react"
import { useAssistance } from "@/hooks/useAssistance"
import { StatusBadge } from "@/components/shared/StatusBadge"
import { SectionHeader } from "@/components/shared/SectionHeader"
import { HospitalityCard } from "@/components/shared/HospitalityCard"
import { relativeTime } from "@/lib/format"
import { Bell, CreditCard, MessageSquare, CheckCircle, Loader2 } from "lucide-react"
import { toast } from "sonner"
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
    <HospitalityCard style={{ padding: "1rem" }}>
      <div className="flex items-start justify-between gap-2">
        <div className="space-y-0.5">
          <p
            className="font-medium text-base leading-snug"
            style={{ fontFamily: "var(--font-display)", color: "var(--color-text)" }}
          >
            {TYPE_LABEL[request.type]} request
          </p>
          <p className="text-xs" style={{ color: "var(--color-text-muted)" }}>
            {relativeTime(request.created_at)}
          </p>
        </div>
        <StatusBadge status={request.status} />
      </div>

      <p className="text-xs mt-3 leading-relaxed" style={{ color: "var(--color-text-muted)" }}>
        {request.status === "pending"
          ? "A staff member has been notified and will be with you shortly."
          : "Someone is on their way to your table."}
      </p>
    </HospitalityCard>
  )
}

export default function AssistPage() {
  const { active, requestAssistance } = useAssistance()
  const [requesting, setRequesting] = useState<AssistanceType | null>(null)

  const activeTypes = new Set(active.map((r) => r.type))

  async function handleRequest(type: AssistanceType) {
    if (activeTypes.has(type)) return
    setRequesting(type)
    try {
      await requestAssistance(type)
      toast.success("Request sent — we'll be right with you.")
    } catch {
      toast.error("Couldn't send request. Please try again.")
    } finally {
      setRequesting(null)
    }
  }

  return (
    <div
      className="px-5 py-7 space-y-7"
      style={{ backgroundColor: "var(--color-bg)", color: "var(--color-text)" }}
    >
      <div className="space-y-1.5">
        <h1
          className="text-3xl font-medium"
          style={{ fontFamily: "var(--font-display)", color: "var(--color-text)" }}
        >
          Need something?
        </h1>
        <p className="text-sm" style={{ color: "var(--color-text-muted)" }}>
          Tap below and a team member will come to you.
        </p>
      </div>

      {active.length > 0 && (
        <section className="space-y-3">
          <SectionHeader>Active requests</SectionHeader>
          {active.map((req) => (
            <ActiveRequestCard key={req.id} request={req} />
          ))}
        </section>
      )}

      <section className="space-y-3">
        {active.length > 0 && <SectionHeader>Request another</SectionHeader>}

        {ASSIST_OPTIONS.map(({ type, label, description, icon: Icon }) => {
          const isActive = activeTypes.has(type)
          const isLoading = requesting === type

          return (
            <button
              key={type}
              onClick={() => handleRequest(type)}
              disabled={isActive || requesting !== null}
              className="w-full flex items-center gap-4 text-left transition-opacity active:opacity-70 disabled:cursor-not-allowed"
              style={{
                backgroundColor: isActive ? "var(--color-surface-inset)" : "var(--color-surface)",
                border: "1px solid var(--color-border)",
                boxShadow: isActive ? "none" : "var(--shadow-card)",
                borderRadius: "var(--radius-lg)",
                opacity: isActive ? 0.55 : 1,
                padding: "1rem",
                minHeight: "76px",
              }}
              aria-label={`${label}: ${description}`}
            >
              <div
                className="size-11 rounded-xl flex items-center justify-center shrink-0"
                style={{ backgroundColor: "var(--color-bg)" }}
              >
                {isLoading ? (
                  <Loader2 className="size-5 animate-spin" style={{ color: "var(--color-accent)" }} aria-hidden />
                ) : isActive ? (
                  <CheckCircle className="size-5" style={{ color: "var(--color-success)" }} aria-hidden />
                ) : (
                  <Icon className="size-5" style={{ color: "var(--color-accent)" }} aria-hidden />
                )}
              </div>
              <div className="flex-1 space-y-0.5">
                <p
                  className="font-medium text-base leading-snug"
                  style={{ fontFamily: "var(--font-display)", color: "var(--color-text)", fontSize: "16px" }}
                >
                  {label}
                </p>
                <p className="text-xs leading-relaxed" style={{ color: "var(--color-text-muted)" }}>
                  {isActive ? "Request already sent" : description}
                </p>
              </div>
            </button>
          )
        })}
      </section>
    </div>
  )
}
