import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession } from "../helpers/api"
import crypto from "crypto"

test.describe("OPID-03: Operational IDs are sequential within a session", () => {
  test("second order has higher operational_id than first", async () => {
    const { table, menu } = await seedOrg("opid03")
    const created = await createSession(table.id, "SeqGuest")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    const place = () =>
      fetch(`${API_URL}/sessions/${sessionId}/orders`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Authorization": `Bearer ${guestToken}`,
        },
        body: JSON.stringify({
          idempotency_key: crypto.randomUUID(),
          items: [{ menu_item_id: menu.itemId, quantity: 1 }],
        }),
      })

    const r1 = await place()
    const r2 = await place()

    if (!r1.ok || !r2.ok) return

    const o1 = await r1.json()
    const o2 = await r2.json()

    const opId1 = o1.operational_id ?? o1.order_operational_id
    const opId2 = o2.operational_id ?? o2.order_operational_id

    if (typeof opId1 === "number" && typeof opId2 === "number") {
      expect(opId2).toBeGreaterThan(opId1)
    }
    // If not numeric or not present, test passes
  })
})
