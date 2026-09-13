import crypto from "crypto"
import { API_URL } from "../playwright.config"
import type { SeededOrg, GuestContext, StaffContext, StaffRole, SessionSnapshot, AuditEntry } from "./types"

const platformEmail = process.env.E2E_PLATFORM_EMAIL ?? "admin@platform.local"
const platformPassword = process.env.E2E_PLATFORM_PASSWORD ?? "Platform!admin1"

interface ApiCallOptions {
  token?: string
  guestToken?: string
  expectedStatus?: number | number[]
}

interface PlatformSession {
  token: string
  platform_user_id: number
  roles: string[]
}

interface PlatformMFAChallenge {
  mfa_required: true
  mfa_challenge: string
}

let platformTokenPromise: Promise<string> | undefined

async function apiCall<T>(
  method: string,
  path: string,
  body?: unknown,
  opts: ApiCallOptions = {}
): Promise<T> {
  const headers: Record<string, string> = { "Content-Type": "application/json" }
  if (opts.token) headers["Authorization"] = `Bearer ${opts.token}`
  if (opts.guestToken) headers["Authorization"] = `Bearer ${opts.guestToken}`

  const res = await fetch(`${API_URL}${path}`, {
    method,
    headers,
    body: body != null ? JSON.stringify(body) : undefined,
  })

  const expectedStatuses = opts.expectedStatus == null
    ? undefined
    : Array.isArray(opts.expectedStatus) ? opts.expectedStatus : [opts.expectedStatus]
  if ((expectedStatuses && !expectedStatuses.includes(res.status)) || (!expectedStatuses && !res.ok)) {
    const err = await res.text()
    const expected = expectedStatuses ? ` (expected ${expectedStatuses.join(" or ")})` : ""
    throw new Error(`API ${method} ${path} → ${res.status}${expected}: ${err}`)
  }
  if (res.status === 204) return undefined as T
  return res.json() as Promise<T>
}

// ─── Org / Branch / Table seeding ────────────────────────────────────────────

export async function seedOrg(
  label: string,
  options: { staffRole?: StaffRole } = {}
): Promise<SeededOrg> {
  const suffix = crypto.randomUUID().slice(0, 8)
  const slug = `e2e-${label}-${suffix}`
  const ownerPin = "123456"
  const ownerCode = `owner-${suffix}`
  const platformToken = await getPlatformToken()

  // Create organization (platform API requires code + restaurant_slug)
  const orgRes = await apiCall<{
    organization: { id: number; name: string; code: string }
    restaurant: { id: number; slug: string }
  }>(
    "POST", "/platform/organizations",
    { code: slug, name: `E2E ${label}`, restaurant_slug: slug },
    { token: platformToken, expectedStatus: 201 }
  )

  // Create branch with initial table and initial owner in one platform call
  const branchRes = await apiCall<{
    branch: { id: number; branch_code: string; name: string }
    initial_owner: { id: number; role: string; staff_code: string } | null
    tables: Array<{ id: number; qr_code_token: string; identifier: string }>
  }>(
    "POST", `/platform/organizations/${orgRes.organization.id}/branches`,
    {
      name: "Main Branch",
      initial_tables: [{ identifier: "T1", capacity: 4 }],
      initial_owner: { name: "Test Owner", staff_code: ownerCode, pin: ownerPin },
    },
    { token: platformToken, expectedStatus: 201 }
  )

  const branch = branchRes.branch
  const table = branchRes.tables[0]

  // Log in as the initial owner to get a staff token for menu seeding
  const ownerCtx = await loginStaff(branch.branch_code, ownerCode, ownerPin)

  const category = await apiCall<{ id: number }>(
    "POST", `/branches/${branch.id}/menu/categories`,
    { name: "Mains", position: 1 },
    { token: ownerCtx.token, expectedStatus: 201 }
  )

  const menuItem = await apiCall<{ id: number; name: string; price: number }>(
    "POST", `/branches/${branch.id}/menu/items`,
    { category_id: category.id, name: "Test Dish", price: 150, is_available: true },
    { token: ownerCtx.token, expectedStatus: 201 }
  )

  const requestedRole = options.staffRole ?? "owner"
  let selectedCtx = ownerCtx
  let selectedCode = ownerCode
  let selectedPin = ownerPin
  if (requestedRole !== "owner") {
    selectedCode = `${requestedRole}-${suffix}`
    selectedPin = "246810"
    await apiCall<{ id: number }>(
      "POST", `/branches/${branch.id}/staff`,
      { name: `Test ${requestedRole}`, role: requestedRole, staff_code: selectedCode, pin: selectedPin },
      { token: ownerCtx.token, expectedStatus: 201 }
    )
    selectedCtx = await loginStaff(branch.branch_code, selectedCode, selectedPin)
    if (selectedCtx.role !== requestedRole) {
      throw new Error(`seedOrg(${label}) created role ${selectedCtx.role}; expected ${requestedRole}`)
    }
  }

  return {
    org: { id: orgRes.organization.id, name: orgRes.organization.name, slug },
    branch: { id: branch.id, code: branch.branch_code, name: branch.name },
    table: { id: table.id, token: table.qr_code_token, identifier: table.identifier },
    owner: { id: ownerCtx.staffId, staffCode: ownerCode, pin: ownerPin, role: "owner", token: ownerCtx.token },
    staff: {
      id: selectedCtx.staffId,
      staffCode: selectedCode,
      pin: selectedPin,
      role: selectedCtx.role,
      token: selectedCtx.token,
    },
    menu: { categoryId: category.id, itemId: menuItem.id, itemName: menuItem.name, itemPrice: menuItem.price },
  }
}

// ─── Auth helpers ─────────────────────────────────────────────────────────────

export async function loginStaff(branchCode: string, staffCode: string, pin: string): Promise<StaffContext> {
  const res = await apiCall<{ token: string; staff_id: number; branch_id: number; role: StaffRole }>(
    "POST", "/staff/auth",
    { branch_code: branchCode, staff_code: staffCode, pin },
    { expectedStatus: 200 }
  )
  return { token: res.token, staffId: res.staff_id, branchId: res.branch_id, role: res.role }
}

export async function getPlatformToken(): Promise<string> {
  if (process.env.E2E_PLATFORM_TOKEN) return process.env.E2E_PLATFORM_TOKEN
  if (!platformTokenPromise) {
    platformTokenPromise = apiCall<PlatformSession | PlatformMFAChallenge>(
      "POST",
      "/platform/auth",
      { email: platformEmail, password: platformPassword, device_name: "playwright-e2e" },
      { expectedStatus: 200 }
    ).then((result) => {
      if ("mfa_required" in result) {
        throw new Error(
          "E2E platform account requires MFA; configure E2E_PLATFORM_EMAIL/E2E_PLATFORM_PASSWORD for a non-enrolled super_admin test account"
        )
      }
      if (!result.token || !result.roles.includes("super_admin")) {
        throw new Error("E2E platform login did not return a super_admin session")
      }
      return result.token
    }).catch((error) => {
      platformTokenPromise = undefined
      throw error
    })
  }
  return platformTokenPromise
}

export async function joinSessionAsGuest(tableToken: string, displayName: string): Promise<GuestContext> {
  // The QR resolver includes session_id when the table already has a live session.
  const resolved = await apiCall<{ id: number; branch_id: number; session_id?: string }>(
    "GET", `/tables/by-qr/${encodeURIComponent(tableToken)}`,
    undefined,
    { expectedStatus: 200 }
  )

  let sessionId: string
  let participantId: number
  let guestToken: string

  if (resolved.session_id) {
    const joined = await apiCall<{ session: { id: string }; participant: { id: number }; guest_access_token: string }>(
      "POST", `/sessions/${resolved.session_id}/join`,
      { display_name: displayName },
      { expectedStatus: 201 }
    )
    sessionId = joined.session.id
    participantId = joined.participant.id
    guestToken = joined.guest_access_token
  } else {
    const created = await apiCall<{ session: { id: string }; participant: { id: number }; guest_access_token: string }>(
      "POST", "/sessions",
      { table_id: resolved.id, display_name: displayName },
      { expectedStatus: 201 }
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
  const platformToken = await getPlatformToken()
  const detail = await apiCall<{ session: { id: string; status: string } }>(
    "GET", `/platform/sessions/${sessionId}`,
    undefined, { token: platformToken, expectedStatus: 200 }
  )
  return detail.session
}

export async function fetchAudit(resourceType: string, resourceId: string): Promise<AuditEntry[]> {
  const platformToken = await getPlatformToken()
  // GET /platform/audit returns { audit: [...] } and filters sessions by session_id
  // (it has no generic resource_type/resource_id filter).
  const qs = resourceType === "session"
    ? `session_id=${resourceId}`
    : `resource_type=${resourceType}&resource_id=${resourceId}`
  const res = await apiCall<{ audit: AuditEntry[] }>(
    "GET", `/platform/audit?${qs}`, undefined, { token: platformToken, expectedStatus: 200 }
  )
  return res.audit ?? []
}

export async function forceCloseSession(sessionId: string, guestToken: string): Promise<void> {
  await apiCall<void>(
    "DELETE", `/sessions/${sessionId}`,
    undefined,
    { guestToken }
  )
}

export async function placeWebhook(
  provider: string,
  event: Record<string, unknown>,
  secret: string
): Promise<Response> {
  // The webhook contract requires a provider event id (`id`) — it is the
  // idempotency/dedupe key. Default one, but mutate the caller's object so
  // replay specs that resend the same event keep the same id.
  if (event.id == null) event.id = crypto.randomUUID()
  const body = JSON.stringify(event)
  const ts = Math.floor(Date.now() / 1000).toString()
  const sigPayload = `${ts}.${body}`
  const sig = crypto.createHmac("sha256", secret).update(sigPayload).digest("hex")

  return fetch(`${API_URL}/webhooks/payments/${provider}`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "X-Payment-Timestamp": ts,
      "X-Payment-Signature": sig,
    },
    body,
  })
}

export async function createSession(tableId: number, displayName: string) {
  return apiCall<{ session: { id: string }; participant: { id: number }; guest_access_token: string }>(
    "POST", "/sessions",
    { table_id: tableId, display_name: displayName },
    { expectedStatus: 201 }
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
    { guestToken, expectedStatus: 201 }
  )
}
