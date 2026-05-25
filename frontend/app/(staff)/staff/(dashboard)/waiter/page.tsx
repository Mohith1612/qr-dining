"use client"

import { useEffect, useState, useCallback } from "react"
import { useStaffStore } from "@/store/staff"
import { assistanceApi } from "@/lib/api/assistance"
import { StatusBadge } from "@/components/shared/StatusBadge"
import { SectionHeader } from "@/components/shared/SectionHeader"
import { EmptyState } from "@/components/shared/EmptyState"
import { HospitalityCard } from "@/components/shared/HospitalityCard"
import { relativeTime } from "@/lib/format"
import { CheckCircle, Loader2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { toast } from "sonner"
import type { AssistanceRequest, AssistanceType } from "@/types/api"

const TYPE_LABEL: Record<AssistanceType, string> = {
  waiter: "Waiter",
  bill: "Bill",
  other: "Other",
}

function RequestCard({
  request,
  onAction,
  acting,
}: {
  request: AssistanceRequest
  onAction: (id: number, action: "ack" | "resolve") => void
  acting: boolean
}) {
  return (
    <HospitalityCard style={{ padding: "1rem" }}>
      <div className="space-y-3">
        <div className="flex items-start justify-between gap-2">
          <div className="space-y-0.5">
            <p
              className="font-medium text-base leading-snug"
              style={{ fontFamily: "var(--font-display)", color: "var(--color-text)", fontSize: "16px" }}
            >
              Table {request.table_id} · {TYPE_LABEL[request.type]}
            </p>
            <p className="text-xs" style={{ color: "var(--color-text-muted)" }}>
              {relativeTime(request.created_at)}
            </p>
          </div>
          <StatusBadge status={request.status} />
        </div>

        <div className="flex gap-2">
          {request.status === "pending" && (
            <Button
              size="sm"
              disabled={acting}
              onClick={() => onAction(request.id, "ack")}
              className="flex-1 h-9 rounded-xl text-xs"
              style={{ backgroundColor: "var(--color-accent)", color: "var(--color-accent-fg)" }}
            >
              {acting ? <Loader2 className="size-3.5 animate-spin" /> : "Acknowledge"}
            </Button>
          )}
          {request.status === "acknowledged" && (
            <Button
              size="sm"
              disabled={acting}
              onClick={() => onAction(request.id, "resolve")}
              className="flex-1 h-9 rounded-xl text-xs"
              style={{ backgroundColor: "var(--color-accent)", color: "var(--color-accent-fg)" }}
            >
              {acting ? <Loader2 className="size-3.5 animate-spin" /> : "Resolve"}
            </Button>
          )}
        </div>
      </div>
    </HospitalityCard>
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

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[40vh]">
        <Loader2 className="size-6 animate-spin" style={{ color: "var(--color-text-muted)" }} />
      </div>
    )
  }

  if (requests.length === 0) {
    return (
      <div style={{ backgroundColor: "var(--color-bg)" }} className="min-h-[60vh]">
        <EmptyState
          icon={CheckCircle}
          title="All clear"
          description="No active requests right now."
        />
      </div>
    )
  }

  return (
    <div
      className="px-5 py-6 space-y-6"
      style={{ backgroundColor: "var(--color-bg)", color: "var(--color-text)" }}
    >
      <h1
        className="text-2xl font-medium"
        style={{ fontFamily: "var(--font-display)", color: "var(--color-text)" }}
      >
        Assistance queue
      </h1>

      {pending.length > 0 && (
        <section className="space-y-3">
          <SectionHeader>Needs attention</SectionHeader>
          {pending.map((r) => (
            <RequestCard
              key={r.id}
              request={r}
              onAction={handleAction}
              acting={acting === r.id}
            />
          ))}
        </section>
      )}

      {acknowledged.length > 0 && (
        <section className="space-y-3">
          <SectionHeader>In progress</SectionHeader>
          {acknowledged.map((r) => (
            <RequestCard
              key={r.id}
              request={r}
              onAction={handleAction}
              acting={acting === r.id}
            />
          ))}
        </section>
      )}
    </div>
  )
}
