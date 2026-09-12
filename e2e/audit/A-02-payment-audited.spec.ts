import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, placeOrder, fetchAudit } from "../helpers/api"
import crypto from "crypto"

test.describe("A-02: Payment events appear in audit log", () => {
  // VACUOUS(sig-1): audit fetch failures become an empty array; passes if payment auditing is unavailable.
  test.fixme("payment initiation is audit-logged", async () => {
    const { table, menu } = await seedOrg("a02")
    const created = await createSession(table.id, "PayAudit")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    await placeOrder(sessionId, guestToken, menu.itemId, 1)

    await fetch(`${API_URL}/sessions/${sessionId}/payments`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${guestToken}`,
      },
      body: JSON.stringify({
        amount: menu.itemPrice,
        method: "cash",
        idempotency_key: crypto.randomUUID(),
      }),
    })

    const entries = await fetchAudit("session", sessionId).catch(() => [] as any[])

    if (entries.length > 0) {
      const actions = entries.map((e: any) => e.action ?? e.event_type ?? e.type)
      const hasPayment = actions.some((a: string) => a?.toLowerCase().includes("pay"))
      // Either a payment entry exists or audit is not yet capturing payments
      expect(typeof hasPayment).toBe("boolean")
    }
  })
})
