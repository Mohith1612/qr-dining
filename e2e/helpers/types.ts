export interface SeededOrg {
  org: { id: number; name: string; slug: string }
  branch: { id: number; code: string; name: string }
  table: { id: number; token: string; identifier: string }
  staff: { id: number; staffCode: string; pin: string; role: string }
  menu: { categoryId: number; itemId: number; itemName: string; itemPrice: number }
}

export interface GuestContext {
  sessionId: string
  participantId: number
  guestToken: string
  tableToken: string
}

export interface StaffContext {
  token: string
  staffId: number
  branchId: number
  role: string
}

export interface SessionSnapshot {
  session: {
    id: string
    status: string
    branch_id: number
    table_id: number
  }
  participants: Array<{
    id: number
    display_name: string
    is_host: boolean
  }>
  missed_events?: unknown[]
}

export interface AuditEntry {
  id: string
  resource_type: string
  resource_id: string
  action: string
  actor_type: string
  actor_id: string
  created_at: string
  details: Record<string, unknown>
}
