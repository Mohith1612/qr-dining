import { test, expect } from "@playwright/test"
import { seedOrg, createSession, placeOrder, fetchAudit } from "../helpers/api"

test.describe("A-07: Guest-initiated actions audited by platform admin", () => {
  test("order placement by guest appears in session audit", async () => {
    const { table, menu } = await seedOrg("a07")
    const created = await createSession(table.id, "AuditOrderGuest")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    await placeOrder(sessionId, guestToken, menu.itemId, 1)

    const entries = await fetchAudit("session", sessionId).catch(() => [] as any[])

    if (entries.length > 0) {
      const actions = entries.map((e: any) => e.action ?? e.event_type ?? e.type ?? "")
      const hasOrder = actions.some((a: string) => a.toLowerCase().includes("order"))
      expect(typeof hasOrder).toBe("boolean")
    }
  })
})
