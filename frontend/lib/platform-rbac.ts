import type { PlatformRole } from "@/types/platform"

// hasPlatformRole mirrors the backend services.PlatformHasRole: super_admin
// bypasses all checks; otherwise the session must hold one of the named roles.
// UI uses this to gate mutation affordances; the backend remains authoritative.
export function hasPlatformRole(roles: PlatformRole[], ...required: PlatformRole[]): boolean {
  if (roles.includes("super_admin")) return true
  if (required.length === 0) return false
  return required.some((r) => roles.includes(r))
}

export function isSuperAdmin(roles: PlatformRole[]): boolean {
  return roles.includes("super_admin")
}

// Reads are allowed for all platform roles (support/billing/auditor + super).
export function canRead(roles: PlatformRole[]): boolean {
  return hasPlatformRole(roles, "support_admin", "billing_admin", "read_only_auditor")
}

const ROLE_LABELS: Record<PlatformRole, string> = {
  super_admin: "Super Admin",
  support_admin: "Support Admin",
  billing_admin: "Billing Admin",
  read_only_auditor: "Read-only Auditor",
}

export function roleLabel(role: PlatformRole): string {
  return ROLE_LABELS[role] ?? role
}
