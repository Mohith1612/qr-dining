import crypto from "crypto"
import { API_URL } from "../playwright.config"
import type { SeededOrg, GuestContext, StaffContext, SessionSnapshot, AuditEntry } from "./types"

const adminToken = process.env.E2E_ADMIN_TOKEN ?? "e2e-admin-secret"

async function apiCall<T>(
  method: string,
  path: string,
  body?: unknown,
  opts: { token?: string; guestToken?: string } = {}
): Promise<T> {
  const headers: Record<string, string> = { "Content-Type": "application/json" }
  if (opts.token) headers["Authorization"] = `Bearer ${opts.token}`
  if (opts.guestToken) headers["Authorization"] = `Bearer ${opts.guestToken}`

  const res = await fetch(`${API_URL}${path}`, {
    method,
    headers,
    body: body != null ? JSON.stringify(body) : undefined,
  })

  if (!res.ok) {
    const err = await res.text()
    throw new Error(`API ${method} ${path} → ${res.status}: ${err}`)
  }
  if (res.status === 204) return undefined as T
  return res.json() as Promise<T>
}

// ─── Org / Branch / Table seeding ────────────────────────────────────────────

export async function seedOrg(label: string): Promise<SeededOrg> {
  const slug = `e2e-${label}-${Date.now()}`

  const org = await apiCall<{ id: number; name: string; slug: string }>(
    "POST", "/platform/organizations",
    { name: `E2E ${label}`, slug },
    { token: adminToken }
  )

  const branch = await apiCall<{ id: number; code: string; name: string }>(
    "POST", `/platform/organizations/${org.id}/branches`,
    { name: "Main Branch", code: slug },
    { token: adminToken }
  )

  const table = await apiCall<{ id: number; token: string; identifier: string }>(
    "POST", `/branches/${branch.id}/tables`,
    { identifier: "T1" },
    { token: adminToken }
  )

  const staffPin = "123456"
  const staffCode = `staff-${Date.now()}`
  const staff = await apiCall<{ id: number; role: string }>(
    "POST", `/branches/${branch.id}/staff`,
    { name: "Test Staff", role: "waiter", staff_code: staffCode, pin: staffPin },
    { token: adminToken }
  )

  const category = await apiCall<{ id: number }>(
    "POST", `/branches/${branch.id}/menu/categories`,
    { name: "Mains", position: 1 },
    { token: adminToken }
  )

  const menuItem = await apiCall<{ id: number; name: string; price: number }>(
    "POST", `/branches/${branch.id}/menu/items`,
    { category_id: category.id, name: "Test Dish", price: 150, is_available: true },
    { token: adminToken }
  )

  return {
    org,
    branch,
    table,
    staff: { id: staff.id, staffCode, pin: staffPin, role: staff.role },
    menu: { categoryId: category.id, itemId: menuItem.id, itemName: menuItem.name, itemPrice: menuItem.price },
  }
}

// ─── Auth helpers ─────────────────────────────────────────────────────────────

export async function loginStaff(branchCode: string, staffCode: string, pin: string): Promise<StaffContext> {
  const res = await apiCall<{ token: string; staff_id: number; branch_id: number; role: string }>(
    "POST", "/staff/auth",
    { branch_code: branchCode, staff_code: staffCode, pin }
  )
  return { token: res.token, staffId: res.staff_id, branchId: res.branch_id, role: res.role }
}

export async function joinSessionAsGuest(tableToken: string, displayName: string): Promise<GuestContext> {
  // Resolve table token → session create or join
  const resolved = await apiCall<{ table_id: number; branch_id: number }>(
    "GET", `/tables/resolve?token=${tableToken}`
  )

  const existing = await apiCall<{ session_id: string } | null>(
    "GET", `/tables/${resolved.table_id}/active-session`
  ).catch(() => null)

  let sessionId: string
  let participantId: number
  let guestToken: string

  if (existing?.session_id) {
    const joined = await apiCall<{ session: { id: string }; participant: { id: number }; guest_access_token: string }>(
      "POST", `/sessions/${existing.session_id}/join`,
      { display_name: displayName }
    )
    sessionId = joined.session.id
    participantId = joined.participant.id
    guestToken = joined.guest_access_token
  } else {
    const created = await apiCall<{ session: { id: string }; participant: { id: number }; guest_access_token: string }>(
      "POST", "/sessions",
      { table_id: resolved.table_id, display_name: displayName }
    )
    sessionId = created.session.id
    participantId = created.participant.id
    guestToken = created.guest_access_token
  }

  return { sessionId, participantId, guestToken, tableToken }
}

// ─── Verification helpers ─────────────────────────────────────────────────────

export async function fetchSnapshot(sessionId: string, guestToken?: string): Promise<SessionSnapshot> {
  return apiCall<SessionSnapshot>("GET", `/sessions/${sessionId}/snapshot`, undefined, { guestToken })
}

export async function fetchSessionDB(sessionId: string): Promise<{ id: string; status: string }> {
  return apiCall<{ id: string; status: string }>(
    "GET", `/platform/sessions/${sessionId}`,
    undefined, { token: adminToken }
  )
}

export async function fetchAudit(resourceType: string, resourceId: string): Promise<AuditEntry[]> {
  return apiCall<AuditEntry[]>(
    "GET", `/platform/audit?resource_type=${resourceType}&resource_id=${resourceId}`,
    undefined, { token: adminToken }
  )
}

export async function forceCloseSession(sessionId: string): Promise<void> {
  await apiCall<void>(
    "POST", `/platform/sessions/${sessionId}/force-close`,
    { reason: "e2e_test" },
    { token: adminToken }
  )
}

export async function placeWebhook(
  provider: string,
  event: Record<string, unknown>,
  secret: string
): Promise<Response> {
  const body = JSON.stringify(event)
  const ts = Math.floor(Date.now() / 1000).toString()
  const sigPayload = `${ts}.${body}`
  const sig = crypto.createHmac("sha256", secret).update(sigPayload).digest("hex")

  return fetch(`${API_URL}/webhooks/payments/${provider}`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "X-Webhook-Timestamp": ts,
      "X-Webhook-Signature": `v1=${sig}`,
    },
    body,
  })
}

export async function createSession(tableId: number, displayName: string) {
  return apiCall<{ session: { id: string }; participant: { id: number }; guest_access_token: string }>(
    "POST", "/sessions",
    { table_id: tableId, display_name: displayName }
  )
}

export async function placeOrder(
  sessionId: string,
  guestToken: string,
  itemId: number,
  quantity = 1
) {
  return apiCall(
    "POST", `/sessions/${sessionId}/orders`,
    {
      idempotency_key: crypto.randomUUID(),
      items: [{ menu_item_id: itemId, quantity }],
    },
    { guestToken }
  )
}
