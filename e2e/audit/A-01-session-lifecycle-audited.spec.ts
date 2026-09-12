import { test, expect } from "@playwright/test"
import { seedOrg, createSession, forceCloseSession, fetchAudit } from "../helpers/api"

test.describe("A-01: Session lifecycle events appear in audit log", () => {
  // VACUOUS(sig-1): audit fetch failures become an empty array; passes if the audit endpoint errors.
  test.fixme("session creation and close are audit-logged", async () => {
    const { table } = await seedOrg("a01")
    const created = await createSession(table.id, "AuditGuest")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    await forceCloseSession(sessionId, guestToken)

    const entries = await fetchAudit("session", sessionId).catch(() => [] as any[])

    if (entries.length > 0) {
      const actions = entries.map((e: any) => e.action ?? e.event_type ?? e.type)
      const hasCreated = actions.some((a: string) =>
        a?.toLowerCase().includes("creat") || a?.toLowerCase().includes("open")
      )
      const hasClosed = actions.some((a: string) => a?.toLowerCase().includes("clos"))
      expect(hasCreated || hasClosed).toBe(true)
    }
    // If audit endpoint not implemented, entries is empty — test passes (no assertion failure)
  })
})
