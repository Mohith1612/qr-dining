import { Badge } from "@/components/ui/badge"
import { cn } from "@/lib/utils"
import type { OrderStatus, AssistanceStatus } from "@/types/api"

type Status = OrderStatus | AssistanceStatus

const STATUS_MAP: Record<Status, { label: string; className: string }> = {
  pending: { label: "Pending", className: "bg-yellow-100 text-yellow-800 border-yellow-200" },
  confirmed: { label: "Confirmed", className: "bg-blue-100 text-blue-800 border-blue-200" },
  preparing: { label: "Preparing", className: "bg-orange-100 text-orange-800 border-orange-200" },
  ready: { label: "Ready", className: "bg-green-100 text-green-800 border-green-200" },
  served: { label: "Served", className: "bg-gray-100 text-gray-600 border-gray-200" },
  cancelled: { label: "Cancelled", className: "bg-red-100 text-red-700 border-red-200" },
  acknowledged: { label: "On the way", className: "bg-blue-100 text-blue-800 border-blue-200" },
  resolved: { label: "Resolved", className: "bg-gray-100 text-gray-600 border-gray-200" },
}

interface StatusBadgeProps {
  status: Status
  className?: string
}

export function StatusBadge({ status, className }: StatusBadgeProps) {
  const { label, className: statusClass } = STATUS_MAP[status] ?? {
    label: status,
    className: "bg-gray-100 text-gray-600",
  }

  return (
    <Badge className={cn("border text-xs font-medium", statusClass, className)}>
      {label}
    </Badge>
  )
}
