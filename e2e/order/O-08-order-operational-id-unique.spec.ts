import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession } from "../helpers/api"
import crypto from "crypto"

test.describe("O-08: Order operational ID is unique per session", () => {
  // VACUOUS(sig-2): uniqueness is asserted only when optional fallback fields exist; passes when both are absent.
  test.fixme("two orders in the same session have distinct operational IDs", async () => {
    const { table, menu } = await seedOrg("o08")
    const created = await createSession(table.id, "OpIDGuest")
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

    const [r1, r2] = await Promise.all([place(), place()])
    expect([200, 201]).toContain(r1.status)
    expect([200, 201]).toContain(r2.status)

    const o1 = await r1.json()
    const o2 = await r2.json()

    const opId1 = o1.operational_id ?? o1.order_operational_id
    const opId2 = o2.operational_id ?? o2.order_operational_id

    if (opId1 != null && opId2 != null) {
      expect(opId1).not.toEqual(opId2)
    }
  })
})
