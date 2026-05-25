"use client"

import { useEffect, useState, useCallback } from "react"
import { useStaffStore } from "@/store/staff"
import { assistanceApi } from "@/lib/api/assistance"
import { StatusBadge } from "@/components/shared/StatusBadge"
import { EmptyState } from "@/components/shared/EmptyState"
import { HospitalityCard } from "@/components/shared/HospitalityCard"
import { relativeTime } from "@/lib/format"
import { CheckCircle, Loader2 } from "lucide-react"
import { toast } from "sonner"
import type { AssistanceRequest, AssistanceType } from "@/types/api"

const TYPE_LABEL: Record<AssistanceType, string> = {
  waiter: "Waiter",
  bill: "Bill",
  other: "Other",
}

const KNOWN_TABLES = [
  { id: 1, label: "T1" },
  { id: 2, label: "T2" },
  { id: 3, label: "T3" },
]

function isUrgent(r: AssistanceRequest) {
  return r.status === "pending" && Date.now() - new Date(r.created_at).getTime() > 5 * 60 * 1000
}

function RequestCard({
  request, onAction, acting,
}: {
  request: AssistanceRequest
  onAction: (id: number, action: "ack" | "resolve") => void
  acting: boolean
}) {
  const urgent = isUrgent(request)
  const borderColor = urgent
    ? "var(--alert)"
    : request.status === "pending" ? "var(--accent)" : "var(--ok)"

  return (
    <div style={{
      background: "var(--bg-elev-1)",
      boxShadow: "var(--shadow-1)",
      borderTop: "1px solid var(--line-1)",
      borderRight: "1px solid var(--line-1)",
      borderBottom: "1px solid var(--line-1)",
      borderLeft: `3px solid ${borderColor}`,
      borderRadius: "var(--rad-lg)",
      overflow: "hidden",
      padding: "14px 16px",
    }}>
      {urgent && (
        <p className="eyebrow" style={{ color: "var(--alert)", marginBottom: 6, margin: "0 0 6px" }}>Urgent</p>
      )}
      <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 8 }}>
        <span style={{
          fontSize: 10, fontWeight: 700, padding: "2px 8px",
          background: "var(--bg-elev-2)", borderRadius: "var(--rad-pill)",
          color: "var(--ink-2)", letterSpacing: "0.04em", textTransform: "uppercase",
        }}>
          Table {request.table_id}
        </span>
        <span style={{ fontSize: 13, color: "var(--ink-2)", fontWeight: 500 }}>
          {TYPE_LABEL[request.type]}
        </span>
        <span style={{ fontSize: 11, color: "var(--ink-4)", marginLeft: "auto" }}>
          {relativeTime(request.created_at)}
        </span>
      </div>
      <StatusBadge status={request.status} />
      <div style={{ display: "flex", gap: 8, marginTop: 12 }}>
        {request.status === "pending" && (
          <button
            disabled={acting}
            onClick={() => onAction(request.id, "ack")}
            className="press"
            style={{
              flex: 1, height: 36,
              background: "var(--accent)", color: "var(--accent-ink)",
              border: "none", borderRadius: "var(--rad-md)",
              fontSize: 12, fontWeight: 600,
              cursor: acting ? "not-allowed" : "pointer",
              display: "flex", alignItems: "center", justifyContent: "center",
              opacity: acting ? 0.6 : 1,
            }}
          >
            {acting
              ? <Loader2 className="animate-spin" style={{ width: 12, height: 12 }} />
              : "Acknowledge"
            }
          </button>
        )}
        {request.status === "acknowledged" && (
          <button
            disabled={acting}
            onClick={() => onAction(request.id, "resolve")}
            className="press"
            style={{
              flex: 1, height: 36,
              background: "var(--ok)", color: "white",
              border: "none", borderRadius: "var(--rad-md)",
              fontSize: 12, fontWeight: 600,
              cursor: acting ? "not-allowed" : "pointer",
              display: "flex", alignItems: "center", justifyContent: "center",
              opacity: acting ? 0.6 : 1,
            }}
          >
            {acting
              ? <Loader2 className="animate-spin" style={{ width: 12, height: 12 }} />
              : "Resolve"
            }
          </button>
        )}
      </div>
    </div>
  )
}

export default function WaiterPage() {
  const { branchId, token } = useStaffStore()
  const [requests, setRequests] = useState<AssistanceRequest[]>([])
  const [loading, setLoading] = useState(true)
  const [acting, setActing] = useState<number | null>(null)

  const fetchRequests = useCallback(async () => {
    if (!branchId || !token) return
    try {
      const data = await assistanceApi.getActive(branchId, token)
      setRequests(data)
    } catch {
      // silent refresh failure — stale data is acceptable
    } finally {
      setLoading(false)
    }
  }, [branchId, token])

  useEffect(() => {
    fetchRequests()
    const interval = setInterval(fetchRequests, 10_000)
    return () => clearInterval(interval)
  }, [fetchRequests])

  async function handleAction(id: number, action: "ack" | "resolve") {
    if (!token) return
    setActing(id)
    try {
      const updated =
        action === "ack"
          ? await assistanceApi.acknowledge(id, token)
          : await assistanceApi.resolve(id, token)
      setRequests((prev) =>
        action === "resolve"
          ? prev.filter((r) => r.id !== id)
          : prev.map((r) => (r.id === id ? updated : r))
      )
      toast.success(action === "ack" ? "Acknowledged" : "Resolved")
    } catch {
      toast.error("Action failed. Please try again.")
    } finally {
      setActing(null)
    }
  }

  const pending = requests.filter((r) => r.status === "pending")
  const acknowledged = requests.filter((r) => r.status === "acknowledged")
  const tableIdsWithRequests = new Set(requests.map((r) => r.table_id))

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[40vh]" style={{ background: "var(--bg-base)" }}>
        <Loader2 className="animate-spin" style={{ width: 24, height: 24, color: "var(--ink-3)" }} />
      </div>
    )
  }

  return (
    <div style={{ background: "var(--bg-base)", color: "var(--ink-1)", minHeight: "100vh" }}>
      <div style={{ padding: "20px 16px", maxWidth: 960, margin: "0 auto" }}>
        {/* Header */}
        <p className="eyebrow">At your service</p>
        <div style={{ display: "flex", alignItems: "center", gap: 10, margin: "4px 0 0" }}>
          <h1 className="serif" style={{ fontSize: 28, fontWeight: 500, color: "var(--ink-1)", margin: 0 }}>
            Request queue
          </h1>
          <span className="live-dot" />
        </div>

        {/* Metrics */}
        <div style={{ display: "flex", gap: 10, marginTop: 12, flexWrap: "wrap" }}>
          {[
            { label: "Total",       value: requests.length  },
            { label: "Pending",     value: pending.length   },
            { label: "In progress", value: acknowledged.length },
          ].map(({ label, value }) => (
            <div key={label} style={{
              background: "var(--bg-elev-1)", border: "1px solid var(--line-2)",
              borderRadius: "var(--rad-pill)", padding: "6px 14px",
              display: "flex", gap: 8, alignItems: "center",
            }}>
              <p className="eyebrow" style={{ margin: 0 }}>{label}</p>
              <span className="serif" style={{ fontSize: 16, fontWeight: 600, color: "var(--ink-1)" }}>{value}</span>
            </div>
          ))}
        </div>

        {/* 2-col layout */}
        <div className="grid grid-cols-1 md:grid-cols-[1.1fr_1fr] gap-6" style={{ marginTop: 24 }}>
          {/* Left: request queue */}
          <div>
            {requests.length === 0 ? (
              <EmptyState icon={CheckCircle} title="All clear" description="No active requests right now." />
            ) : (
              <div>
                <p className="eyebrow" style={{ marginBottom: 10 }}>
                  Needs attention ({requests.length})
                </p>
                <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
                  {[...pending, ...acknowledged].map((r) => (
                    <RequestCard
                      key={r.id}
                      request={r}
                      onAction={handleAction}
                      acting={acting === r.id}
                    />
                  ))}
                </div>
              </div>
            )}
          </div>

          {/* Right: floor overview */}
          <div>
            <p className="eyebrow" style={{ marginBottom: 10 }}>Floor</p>
            <div className="grid grid-cols-2 gap-3">
              {KNOWN_TABLES.map(({ id, label }) => {
                const hasRequest = tableIdsWithRequests.has(id)
                const dotColor = hasRequest ? "var(--accent)" : "var(--ok-soft)"
                return (
                  <HospitalityCard key={id} elev={1} style={{ padding: "12px 14px" }}>
                    <div style={{ display: "flex", alignItems: "center", gap: 6, marginBottom: 4 }}>
                      <div style={{ width: 8, height: 8, borderRadius: "50%", background: dotColor, flexShrink: 0 }} />
                      <span className="serif" style={{ fontSize: 15, fontWeight: 500, color: "var(--ink-2)" }}>
                        {label}
                      </span>
                    </div>
                    <span style={{ fontSize: 10, color: "var(--ink-4)" }}>
                      {hasRequest ? "Active request" : "No requests"}
                    </span>
                  </HospitalityCard>
                )
              })}
            </div>

            {/* Legend */}
            <div style={{ display: "flex", gap: 12, marginTop: 12, flexWrap: "wrap" }}>
              {[
                { label: "Active request", color: "var(--accent)" },
                { label: "No requests",    color: "var(--ok-soft)" },
              ].map(({ label, color }) => (
                <div key={label} style={{ display: "flex", alignItems: "center", gap: 4 }}>
                  <div style={{ width: 6, height: 6, borderRadius: "50%", background: color }} />
                  <span style={{ fontSize: 10, color: "var(--ink-4)" }}>{label}</span>
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
