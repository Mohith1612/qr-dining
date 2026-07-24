"use client"

import { useEffect, useState, useCallback } from "react"
import { useStaffStore } from "@/store/staff"
import { staffApi } from "@/lib/api/staff"
import { menuApi } from "@/lib/api/menu"
import { analyticsApi, type AnalyticsPeriod, type TopItem, type BusyHour, type OrderVolumeDay } from "@/lib/api/analytics"
import { plansApi, type Subscription, type Plan } from "@/lib/api/plans"
import { ApiError } from "@/lib/api/client"
import { useTenant } from "@/providers/TenantProvider"
import { PeriodSelector } from "@/components/shared/PeriodSelector"
import { TopItemsList } from "@/components/analytics/TopItemsList"
import { BusyHoursChart } from "@/components/analytics/BusyHoursChart"
import { OrderVolumeChart } from "@/components/analytics/OrderVolumeChart"
import { HospitalityCard } from "@/components/shared/HospitalityCard"
import { SectionHeader } from "@/components/shared/SectionHeader"
import { EmptyState } from "@/components/shared/EmptyState"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { cn } from "@/lib/utils"
import { formatCurrency, relativeTime } from "@/lib/format"
import { RefreshCw, Loader2, Users, BarChart2, CreditCard, Printer, MoreVertical, RotateCcw, QrCode, Settings2, ChevronDown, ChevronRight, Plus, Trash2, Edit2, Tag, KeyRound, UserX } from "lucide-react"
import { toast } from "sonner"
import Link from "next/link"
import type { Session, MenuCategory, MenuItem, StaffRole, StaffRosterMember, Table, Promo } from "@/types/api"
import { promosApi } from "@/lib/api/promos"
import { tablesApi } from "@/lib/api/tables"
import { themeApi } from "@/lib/api/theme"
import { QRCard } from "@/components/admin/QRCard"
import { PrintTemplate } from "@/components/admin/PrintTemplate"
import { CollateralStudio } from "@/components/collateral/CollateralStudio"
import type { ThemeConfig } from "@/lib/theme/applyTheme"
import type { CollateralBranding, CollateralConfig, CollateralTable } from "@/types/collateral"
import { normalizeCollateralConfig, DEFAULT_COLLATERAL } from "@/types/collateral"
import { BottomSheet } from "@/components/shared/BottomSheet"
import { MenuItemModal } from "@/components/admin/MenuItemModal"
import { StatsSkeleton, TablesSkeleton } from "@/components/shared/LoadingSkeleton"

function TableRow({
  table,
  canManage,
  onPrint,
  onRefreshQR,
}: {
  table: Table
  canManage: boolean
  onPrint: (t: Table) => void
  onRefreshQR: (t: Table) => void
}) {
  const isOccupied = table.status === "occupied"

  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        gap: 12,
        padding: "10px 14px",
        background: "var(--bg-elev-1)",
        border: "1px solid var(--line-1)",
        borderRadius: "var(--rad-md)",
      }}
    >
      {/* Identifier + capacity */}
      <div style={{ flex: 1, minWidth: 0 }}>
        <span className="serif" style={{ fontSize: 18, color: "var(--ink-1)", fontWeight: 500 }}>
          {table.identifier}
        </span>
        <span style={{ fontSize: 12, color: "var(--ink-3)", marginLeft: 8 }}>
          {table.capacity} seats
        </span>
      </div>

      {/* Status pill */}
      <span
        style={{
          fontSize: 11,
          fontWeight: 600,
          letterSpacing: "0.06em",
          textTransform: "uppercase",
          padding: "2px 8px",
          borderRadius: 99,
          background: isOccupied ? "var(--warn-soft)" : "var(--ok-soft)",
          color: isOccupied ? "var(--warn)" : "var(--ok)",
          flexShrink: 0,
        }}
      >
        {isOccupied ? "Occupied" : "Available"}
      </span>

      {/* QR preview */}
      <QRCard table={table} size={48} />

      {/* Actions */}
      <button
        onClick={() => onPrint(table)}
        className="press"
        title="Print QR card"
        style={{
          display: "flex",
          alignItems: "center",
          gap: 4,
          fontSize: 12,
          color: "var(--ink-2)",
          padding: "5px 10px",
          borderRadius: "var(--rad-md)",
          background: "var(--bg-elev-2)",
          border: "1px solid var(--line-1)",
          flexShrink: 0,
        }}
      >
        <Printer size={12} aria-hidden />
        Print
      </button>

      {canManage && (
        <button
          onClick={() => onRefreshQR(table)}
          disabled={isOccupied}
          className="press"
          title={isOccupied ? "Cannot regenerate while table is occupied" : "Regenerate QR token"}
          style={{
            display: "flex",
            alignItems: "center",
            gap: 4,
            fontSize: 12,
            padding: "5px 10px",
            borderRadius: "var(--rad-md)",
            background: "var(--bg-elev-2)",
            border: "1px solid var(--line-1)",
            flexShrink: 0,
            color: isOccupied ? "var(--ink-4)" : "var(--ink-2)",
            cursor: isOccupied ? "not-allowed" : "pointer",
            opacity: isOccupied ? 0.5 : 1,
          }}
        >
          <RotateCcw size={12} aria-hidden />
          Regen QR
        </button>
      )}
    </div>
  )
}

export function TablesTab() {
  const { branchId, token, role } = useStaffStore()
  const { slug } = useTenant()
  const [tables, setTables] = useState<Table[]>([])
  const [loading, setLoading] = useState(true)
  const [createModal, setCreateModal] = useState(false)
  const [creating, setCreating] = useState(false)
  const [identifier, setIdentifier] = useState("")
  const [capacity, setCapacity] = useState(4)
  const [printTable, setPrintTable] = useState<Table | null>(null)

  const canManage = role === "owner" || role === "manager"

  const fetchTables = useCallback(async () => {
    if (!branchId || !token) return
    setLoading(true)
    try {
      const data = await tablesApi.list(branchId, token)
      setTables(data)
    } catch {
      toast.error("Couldn't load tables.")
    } finally {
      setLoading(false)
    }
  }, [branchId, token])

  useEffect(() => {
    fetchTables()
  }, [fetchTables])

  async function handleCreate() {
    if (!branchId || !token || !identifier.trim()) return
    setCreating(true)
    try {
      const table = await tablesApi.create(branchId, { identifier: identifier.trim(), capacity }, token)
      setTables(prev => [...prev, table].sort((a, b) => a.identifier.localeCompare(b.identifier)))
      setCreateModal(false)
      setIdentifier("")
      setCapacity(4)
      toast.success(`Table ${table.identifier} created.`)
    } catch {
      toast.error("Failed to create table.")
    } finally {
      setCreating(false)
    }
  }

  async function handleRefreshQR(table: Table) {
    if (!token) return
    if (!confirm(`Regenerate QR for ${table.identifier}? This will invalidate all printed cards.`)) return
    try {
      const updated = await tablesApi.refreshQR(table.id, token)
      setTables(prev => prev.map(t => t.id === updated.id ? updated : t))
      toast.success(`QR token regenerated for ${table.identifier}.`)
    } catch {
      toast.error("Failed to regenerate QR token.")
    }
  }

  function handlePrint(table: Table) {
    setPrintTable(table)
    setTimeout(() => window.print(), 100)
  }

  if (loading) return <TablesSkeleton />

  return (
    <div>
      {/* Header */}
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 16 }}>
        <p className="eyebrow">{tables.length} table{tables.length !== 1 ? "s" : ""}</p>
        {canManage && (
          <button
            onClick={() => setCreateModal(true)}
            className="press"
            style={{
              height: 36,
              padding: "0 16px",
              borderRadius: 10,
              fontSize: 13,
              fontWeight: 600,
              background: "var(--accent)",
              color: "var(--accent-ink)",
              border: "none",
              cursor: "pointer",
            }}
          >
            Add table
          </button>
        )}
      </div>

      {/* Table list */}
      {tables.length === 0 ? (
        <EmptyState icon={QrCode} title="No tables yet" description="Add your first table to generate a QR code." />
      ) : (
        <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
          {tables.map(table => (
            <TableRow
              key={table.id}
              table={table}
              canManage={canManage}
              onPrint={handlePrint}
              onRefreshQR={handleRefreshQR}
            />
          ))}
        </div>
      )}

      {/* Create modal */}
      {createModal && (
        <BottomSheet open onClose={() => setCreateModal(false)} title="Add table">
          <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
            <div>
              <p className="eyebrow" style={{ marginBottom: 6 }}>Table identifier</p>
              <Input
                placeholder="e.g. T3, Table 12, Garden 2"
                value={identifier}
                onChange={e => setIdentifier(e.target.value)}
                onKeyDown={e => e.key === "Enter" && handleCreate()}
              />
            </div>
            <div>
              <p className="eyebrow" style={{ marginBottom: 6 }}>Capacity</p>
              <Input
                type="number"
                min={1}
                max={20}
                value={capacity}
                onChange={e => setCapacity(Number(e.target.value))}
              />
            </div>
            <button
              onClick={handleCreate}
              disabled={creating || !identifier.trim()}
              className="press"
              style={{
                height: 44,
                borderRadius: "var(--rad-md)",
                fontSize: 14,
                fontWeight: 600,
                background: "var(--accent)",
                color: "var(--accent-ink)",
                border: "none",
                cursor: creating || !identifier.trim() ? "not-allowed" : "pointer",
                opacity: creating || !identifier.trim() ? 0.6 : 1,
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                gap: 8,
              }}
            >
              {creating && <Loader2 size={14} className="animate-spin" />}
              Create table
            </button>
          </div>
        </BottomSheet>
      )}

      {/* Hidden print template — rendered when printTable is set */}
      {printTable && <PrintTemplate table={printTable} tenantSlug={slug} />}
    </div>
  )
}

