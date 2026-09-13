"use client"

import { useEffect, useState, useCallback } from "react"
import { useStaffStore } from "@/store/staff"
import { ApiError } from "@/lib/api/client"
import { useTenant } from "@/providers/TenantProvider"
import { EmptyState } from "@/components/shared/EmptyState"
import { Input } from "@/components/ui/input"
import { Loader2, Printer, RotateCcw, QrCode, Trash2, Edit2 } from "lucide-react"
import { toast } from "sonner"
import type { Table } from "@/types/api"
import { tablesApi } from "@/lib/api/tables"
import { QRCard } from "@/components/admin/QRCard"
import { PrintTemplate } from "@/components/admin/PrintTemplate"
import { BottomSheet } from "@/components/shared/BottomSheet"
import { TablesSkeleton } from "@/components/shared/LoadingSkeleton"

function TableRow({
  table,
  canManage,
  onPrint,
  onRefreshQR,
  onEdit,
  onDelete,
}: {
  table: Table
  canManage: boolean
  onPrint: (t: Table) => void
  onRefreshQR: (t: Table) => void
  onEdit: (t: Table) => void
  onDelete: (t: Table) => void
}) {
  const isOccupied = table.status === "occupied"
  const iconBtn: React.CSSProperties = {
    display: "flex", alignItems: "center", justifyContent: "center",
    width: 30, height: 30, borderRadius: "var(--rad-md)",
    background: "var(--bg-elev-2)", border: "1px solid var(--line-1)",
    color: "var(--ink-2)", flexShrink: 0, cursor: "pointer",
  }

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

      {canManage && (
        <>
          <button onClick={() => onEdit(table)} className="press" title="Edit table" aria-label={`Edit ${table.identifier}`} style={iconBtn}>
            <Edit2 size={13} aria-hidden />
          </button>
          <button
            onClick={() => onDelete(table)}
            disabled={isOccupied}
            className="press"
            title={isOccupied ? "Cannot delete while table is occupied" : "Delete table"}
            aria-label={`Delete ${table.identifier}`}
            style={{ ...iconBtn, color: isOccupied ? "var(--ink-4)" : "var(--alert)", cursor: isOccupied ? "not-allowed" : "pointer", opacity: isOccupied ? 0.5 : 1 }}
          >
            <Trash2 size={13} aria-hidden />
          </button>
        </>
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
  const [editTable, setEditTable] = useState<Table | null>(null)
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

  function openCreate() {
    setEditTable(null)
    setIdentifier("")
    setCapacity(4)
    setCreateModal(true)
  }

  function openEdit(table: Table) {
    setEditTable(table)
    setIdentifier(table.identifier)
    setCapacity(table.capacity)
    setCreateModal(true)
  }

  async function handleSubmit() {
    if (!branchId || !token || !identifier.trim()) return
    setCreating(true)
    try {
      if (editTable) {
        const updated = await tablesApi.update(editTable.id, { identifier: identifier.trim(), capacity }, token)
        setTables(prev => prev.map(t => t.id === updated.id ? updated : t).sort((a, b) => a.identifier.localeCompare(b.identifier)))
        toast.success(`Table ${updated.identifier} updated.`)
      } else {
        const table = await tablesApi.create(branchId, { identifier: identifier.trim(), capacity }, token)
        setTables(prev => [...prev, table].sort((a, b) => a.identifier.localeCompare(b.identifier)))
        toast.success(`Table ${table.identifier} created.`)
      }
      setCreateModal(false)
      setEditTable(null)
      setIdentifier("")
      setCapacity(4)
    } catch (err) {
      toast.error(err instanceof ApiError && err.code === "DUPLICATE_TABLE_IDENTIFIER"
        ? "Another table already uses that name."
        : editTable ? "Failed to update table." : "Failed to create table.")
    } finally {
      setCreating(false)
    }
  }

  async function handleDelete(table: Table) {
    if (!token) return
    if (!confirm(`Delete table ${table.identifier}? This can't be undone.`)) return
    try {
      await tablesApi.remove(table.id, token)
      setTables(prev => prev.filter(t => t.id !== table.id))
      toast.success(`Table ${table.identifier} deleted.`)
    } catch (err) {
      toast.error(err instanceof ApiError && err.code === "TABLE_OCCUPIED"
        ? "Can't delete a table with a session in progress."
        : "Failed to delete table.")
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
            onClick={openCreate}
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
              onEdit={openEdit}
              onDelete={handleDelete}
            />
          ))}
        </div>
      )}

      {/* Create / edit modal */}
      {createModal && (
        <BottomSheet open onClose={() => setCreateModal(false)} title={editTable ? "Edit table" : "Add table"}>
          <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
            <div>
              <p className="eyebrow" style={{ marginBottom: 6 }}>Table identifier</p>
              <Input
                placeholder="e.g. T3, Table 12, Garden 2"
                value={identifier}
                onChange={e => setIdentifier(e.target.value)}
                onKeyDown={e => e.key === "Enter" && handleSubmit()}
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
              onClick={handleSubmit}
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
              {editTable ? "Save changes" : "Create table"}
            </button>
          </div>
        </BottomSheet>
      )}

      {/* Hidden print template — rendered when printTable is set */}
      {printTable && <PrintTemplate table={printTable} tenantSlug={slug} />}
    </div>
  )
}

