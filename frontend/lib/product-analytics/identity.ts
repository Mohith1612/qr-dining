import type { StaffRole } from "@/types/api"
import type { PlatformRole } from "@/types/platform"
import { env } from "@/config/env"
import { ensureInit } from "@/lib/product-analytics/posthog"

const GUEST_SESSION_PROPS = ["session_id", "participant_id", "is_host", "table_id"] as const

export function identifyStaff(staffId: number, role: StaffRole, branchId: number): void {
  const client = ensureInit()
  if (!client) return
  client.identify(`staff:${staffId}`, { role, branch_id: branchId })
  client.register({ branch_id: branchId })
}

export function identifyPlatform(userId: number, roles: PlatformRole[]): void {
  ensureInit()?.identify(`platform:${userId}`, { roles })
}

export function resetIdentity(): void {
  const client = ensureInit()
  if (!client) return
  client.reset()
  client.register({
    env: env.posthogEnv,
    ...(env.appVersion ? { app_version: env.appVersion } : {}),
  })
}

export function registerTenantProps(props: { orgId?: number | null; restaurantSlug?: string | null }): void {
  const values: Record<string, string | number> = {}
  if (props.orgId != null) values.org_id = props.orgId
  if (props.restaurantSlug) values.restaurant_slug = props.restaurantSlug
  if (Object.keys(values).length) ensureInit()?.register(values)
}

export function registerBranchProps(branchId: number): void {
  ensureInit()?.register({ branch_id: branchId })
}

export function registerGuestSessionProps(props: {
  sessionId: string
  participantId: number
  isHost: boolean
  tableId: number
}): void {
  ensureInit()?.register_for_session({
    session_id: props.sessionId,
    participant_id: props.participantId,
    is_host: props.isHost,
    table_id: props.tableId,
  })
}

export function unregisterGuestSessionProps(): void {
  const client = ensureInit()
  if (!client) return
  for (const prop of GUEST_SESSION_PROPS) client.unregister_for_session(prop)
}
