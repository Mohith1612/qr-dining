"use client"

import { useEffect, useState, useCallback } from "react"
import { useStaffStore } from "@/store/staff"
import { staffApi } from "@/lib/api/staff"
import { ApiError } from "@/lib/api/client"
import { HospitalityCard } from "@/components/shared/HospitalityCard"
import { EmptyState } from "@/components/shared/EmptyState"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { RefreshCw, Loader2, Users, KeyRound, UserX } from "lucide-react"
import { toast } from "sonner"
import type { StaffRole, StaffRosterMember } from "@/types/api"
import { BottomSheet } from "@/components/shared/BottomSheet"

const STAFF_ROLES: { value: StaffRole; label: string }[] = [
  { value: "waiter",  label: "Waiter"  },
  { value: "kitchen", label: "Kitchen" },
  { value: "manager", label: "Manager" },
]

const ROLE_BADGE: Record<StaffRole, string> = {
  owner:   "Owner",
  manager: "Manager",
  waiter:  "Waiter",
  kitchen: "Kitchen",
}

function StaffRoleBadge({ role }: { role: StaffRole }) {
  return (
    <span style={{
      padding: "2px 8px", borderRadius: 999,
      background: "var(--accent-soft)", color: "var(--accent)",
      fontSize: 10, letterSpacing: "0.08em", textTransform: "uppercase", fontWeight: 600,
    }}>
      {ROLE_BADGE[role]}
    </span>
  )
}

export function StaffTab() {
  const { branchId, token, role, staffId } = useStaffStore()
  const [roster, setRoster] = useState<StaffRosterMember[]>([])
  const [loadingRoster, setLoadingRoster] = useState(true)

  const [name, setName] = useState("")
  const [selectedRole, setSelectedRole] = useState<StaffRole>("waiter")
  const [pin, setPin] = useState("")
  const [creating, setCreating] = useState(false)

  const [resetTarget, setResetTarget] = useState<StaffRosterMember | null>(null)
  const [resetPin, setResetPin] = useState("")
  const [resetting, setResetting] = useState(false)

  const canManage = role === "owner" || role === "manager"
  const isOwner = role === "owner"

  const fetchRoster = useCallback(async () => {
    if (!branchId || !token) return
    setLoadingRoster(true)
    try {
      const data = await staffApi.listStaff(branchId, token)
      setRoster(data.staff)
    } catch (err) {
      // A transient server/network error is retryable, not a real "no roster".
      const transient = !(err instanceof ApiError) || err.status >= 500
      toast.error(transient
        ? "Couldn't load the staff roster — retrying…"
        : "Couldn't load the staff roster.")
      if (transient) {
        setTimeout(() => { void fetchRoster() }, 1500)
      }
    } finally {
      setLoadingRoster(false)
    }
  }, [branchId, token])

  useEffect(() => {
    if (canManage) fetchRoster()
  }, [canManage, fetchRoster])

  // Mirrors the backend: never self; managers may reset only waiter/kitchen.
  function canResetTarget(member: StaffRosterMember): boolean {
    if (member.id === staffId) return false
    if (isOwner) return true
    return member.role === "waiter" || member.role === "kitchen"
  }

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    if (!branchId || !token || !name || !pin) return
    setCreating(true)
    try {
      await staffApi.createStaff(branchId, name, selectedRole, pin, token)
      toast.success(`${name} added as ${selectedRole}`)
      setName("")
      setPin("")
      fetchRoster()
    } catch {
      toast.error("Couldn't create staff account.")
    } finally {
      setCreating(false)
    }
  }

  async function handleResetPin() {
    if (!resetTarget || !token || resetPin.length < 4) return
    setResetting(true)
    try {
      await staffApi.resetPin(resetTarget.id, resetPin, token)
      toast.success(`PIN reset for ${resetTarget.name}`)
      setResetTarget(null)
      setResetPin("")
      fetchRoster()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Couldn't reset PIN.")
    } finally {
      setResetting(false)
    }
  }

  async function handleDeactivate(member: StaffRosterMember) {
    if (!token) return
    if (!window.confirm(`Deactivate ${member.name}? They will be signed out and unable to log in.`)) return
    try {
      await staffApi.deactivate(member.id, token)
      toast.success(`${member.name} deactivated`)
      fetchRoster()
    } catch {
      toast.error("Couldn't deactivate staff member.")
    }
  }

  if (!canManage) {
    return (
      <div style={{ padding: "48px 0", textAlign: "center", color: "var(--ink-3)" }}>
        <p style={{ fontSize: 14 }}>Only owners and managers can manage staff accounts.</p>
      </div>
    )
  }

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
      {/* Roster */}
      <HospitalityCard elev={2} style={{ padding: 20 }}>
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 16 }}>
          <p className="serif" style={{ fontSize: 18, fontWeight: 500, color: "var(--ink-1)" }}>
            Staff
          </p>
          <button
            onClick={fetchRoster}
            className="press"
            aria-label="Refresh roster"
            style={{ display: "inline-flex", alignItems: "center", gap: 6, padding: "6px 10px", borderRadius: "var(--rad-md)", border: "1px solid var(--line-2)", background: "transparent", color: "var(--ink-3)", fontSize: 12 }}
          >
            <RefreshCw size={13} className={loadingRoster ? "animate-spin" : undefined} />
            Refresh
          </button>
        </div>

        {loadingRoster ? (
          <div className="flex items-center justify-center py-10">
            <Loader2 className="size-5 animate-spin opacity-40" />
          </div>
        ) : roster.length === 0 ? (
          <EmptyState icon={Users} title="No staff yet." />
        ) : (
          <div style={{ display: "flex", flexDirection: "column" }}>
            {roster.map((member, i) => {
              const showReset = canResetTarget(member)
              const showDeactivate = isOwner && member.id !== staffId
              return (
                <div key={member.id}>
                  {i > 0 && <hr className="rule" />}
                  <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, padding: "12px 0" }}>
                    <div style={{ minWidth: 0 }}>
                      <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
                        <span style={{ fontSize: 14, fontWeight: 500, color: "var(--ink-1)" }}>{member.name}</span>
                        <StaffRoleBadge role={member.role} />
                        {member.id === staffId && (
                          <span style={{ fontSize: 11, color: "var(--ink-4)" }}>You</span>
                        )}
                      </div>
                      <div className="font-mono" style={{ fontSize: 11, color: "var(--ink-3)", marginTop: 3 }}>
                        {member.staff_code}
                      </div>
                    </div>
                    <div style={{ display: "flex", alignItems: "center", gap: 6, flexShrink: 0 }}>
                      {showReset && (
                        <button
                          onClick={() => { setResetTarget(member); setResetPin("") }}
                          className="press"
                          style={{ display: "inline-flex", alignItems: "center", gap: 5, padding: "7px 11px", borderRadius: "var(--rad-md)", border: "1px solid var(--line-2)", background: "var(--bg-base)", color: "var(--ink-2)", fontSize: 12, fontWeight: 500 }}
                        >
                          <KeyRound size={13} />
                          Reset PIN
                        </button>
                      )}
                      {showDeactivate && (
                        <button
                          onClick={() => handleDeactivate(member)}
                          className="press"
                          aria-label={`Deactivate ${member.name}`}
                          style={{ display: "inline-flex", alignItems: "center", justifyContent: "center", width: 34, height: 34, borderRadius: "var(--rad-md)", border: "1px solid var(--line-2)", background: "var(--bg-base)", color: "var(--danger, var(--ink-3))" }}
                        >
                          <UserX size={14} />
                        </button>
                      )}
                    </div>
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </HospitalityCard>

      {/* Add staff — owners only (backend create is owner-only) */}
      {isOwner && (
        <HospitalityCard elev={2} style={{ padding: 20 }}>
          <p className="serif" style={{ fontSize: 18, fontWeight: 500, color: "var(--ink-1)", marginBottom: 16 }}>
            Add staff member
          </p>

          <form onSubmit={handleCreate} style={{ display: "flex", flexDirection: "column", gap: 16 }}>
            <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
              <label style={{ fontSize: 11, fontWeight: 600, color: "var(--ink-3)", textTransform: "uppercase", letterSpacing: "0.1em" }}>
                Name
              </label>
              <Input
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Staff name"
                required
                style={{
                  height: 44,
                  borderRadius: "var(--rad-lg)",
                  background: "var(--bg-base)",
                  borderColor: "var(--line-2)",
                  color: "var(--ink-1)",
                }}
              />
            </div>

            <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
              <label style={{ fontSize: 11, fontWeight: 600, color: "var(--ink-3)", textTransform: "uppercase", letterSpacing: "0.1em" }}>
                Role
              </label>
              <div style={{ display: "flex", gap: 6 }}>
                {STAFF_ROLES.map(({ value, label }) => (
                  <button
                    key={value}
                    type="button"
                    onClick={() => setSelectedRole(value)}
                    className="press"
                    style={{
                      flex: 1, padding: "10px 8px",
                      borderRadius: "var(--rad-md)",
                      fontSize: 13, fontWeight: 500,
                      border: "1px solid",
                      borderColor: selectedRole === value ? "var(--accent)" : "var(--line-2)",
                      background: selectedRole === value ? "var(--accent-soft)" : "var(--bg-base)",
                      color: selectedRole === value ? "var(--accent)" : "var(--ink-3)",
                      minHeight: 44,
                      transition: "background var(--dur-fast) var(--ease), color var(--dur-fast) var(--ease), border-color var(--dur-fast) var(--ease)",
                    }}
                  >
                    {label}
                  </button>
                ))}
              </div>
            </div>

            <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
              <label style={{ fontSize: 11, fontWeight: 600, color: "var(--ink-3)", textTransform: "uppercase", letterSpacing: "0.1em" }}>
                PIN
              </label>
              <Input
                type="password"
                inputMode="numeric"
                value={pin}
                onChange={(e) => setPin(e.target.value)}
                placeholder="4–6 digits"
                maxLength={6}
                required
                style={{
                  height: 44,
                  borderRadius: "var(--rad-lg)",
                  background: "var(--bg-base)",
                  borderColor: "var(--line-2)",
                  color: "var(--ink-1)",
                  letterSpacing: "0.2em",
                }}
              />
            </div>

            <Button
              type="submit"
              disabled={creating || !name || !pin}
              style={{
                width: "100%", height: 44,
                borderRadius: "var(--rad-lg)",
                background: creating ? "var(--bg-elev-3)" : "linear-gradient(180deg, var(--accent-strong), var(--accent))",
                color: creating ? "var(--ink-3)" : "var(--accent-ink)",
                border: "none", fontSize: 14, fontWeight: 600,
              }}
            >
              {creating ? (
                <span style={{ display: "flex", alignItems: "center", gap: 8 }}>
                  <Loader2 size={16} className="animate-spin" />
                  Creating…
                </span>
              ) : (
                "Create account"
              )}
            </Button>
          </form>
        </HospitalityCard>
      )}

      {/* Reset PIN modal */}
      {resetTarget && (
        <BottomSheet open onClose={() => setResetTarget(null)} title={`Reset PIN — ${resetTarget.name}`}>
          <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
            <p style={{ fontSize: 13, color: "var(--ink-3)" }}>
              Set a new PIN for {resetTarget.name}. Their existing sessions will be signed out.
            </p>
            <div>
              <p className="eyebrow" style={{ marginBottom: 6 }}>New PIN</p>
              <Input
                type="password"
                inputMode="numeric"
                value={resetPin}
                onChange={(e) => setResetPin(e.target.value.replace(/\D/g, ""))}
                placeholder="4–6 digits"
                maxLength={6}
                onKeyDown={(e) => e.key === "Enter" && handleResetPin()}
                style={{ letterSpacing: "0.2em" }}
              />
            </div>
            <button
              onClick={handleResetPin}
              disabled={resetting || resetPin.length < 4}
              className="press"
              style={{
                height: 44, borderRadius: "var(--rad-md)", fontSize: 14, fontWeight: 600,
                background: "var(--accent)", color: "var(--accent-ink)", border: "none",
                cursor: resetting || resetPin.length < 4 ? "not-allowed" : "pointer",
                opacity: resetting || resetPin.length < 4 ? 0.6 : 1,
                display: "flex", alignItems: "center", justifyContent: "center", gap: 8,
              }}
            >
              {resetting && <Loader2 size={14} className="animate-spin" />}
              Reset PIN
            </button>
          </div>
        </BottomSheet>
      )}
    </div>
  )
}

