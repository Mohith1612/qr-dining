import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession } from "../helpers/api"
import crypto from "crypto"

test.describe("R-09: Duplicate event deduplication via idempotency", () => {
  test("same idempotency key returns same order on retry", async () => {
    const { table, menu } = await seedOrg("r09")
    const created = await createSession(table.id, "DedupUser")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token
    const key = crypto.randomUUID()

    const body = JSON.stringify({
      idempotency_key: key,
      items: [{ menu_item_id: menu.itemId, quantity: 1 }],
    })

    // First request
    const res1 = await fetch(`${API_URL}/sessions/${sessionId}/orders`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${guestToken}`,
      },
      body,
    })
    expect(res1.status).toBe(201)
    const order1 = await res1.json()

    // Retry with same key and same body
    const res2 = await fetch(`${API_URL}/sessions/${sessionId}/orders`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${guestToken}`,
      },
      body,
    })
    expect(res2.status).toBe(201)
    const order2 = await res2.json()

    // Should return same order. Read `order.id` directly rather than through a
    // fallback chain: `a?.X ?? a.Y` quietly compares undefined to undefined when
    // both field names are wrong, which is how this assertion survived the
    // PlaceOrderResult casing change (F-24) without failing.
    expect(order1.order.id).toMatch(/^[0-9a-f-]{36}$/)
    expect(order1.order.id).toBe(order2.order.id)

    // Verify only one order exists
    const ordersRes = await fetch(`${API_URL}/sessions/${sessionId}/orders`, {
      headers: { "Authorization": `Bearer ${guestToken}` },
    })
    const orders = await ordersRes.json()
    expect(orders.filter((o: { idempotency_key?: string }) => o.idempotency_key === key).length).toBeLessThanOrEqual(1)
  })
})
