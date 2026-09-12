import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession } from "../helpers/api"
import crypto from "crypto"

test.describe("O-01: Order placement — happy path", () => {
  test("guest places order and receives pending order with operational ID", async () => {
    const { table, menu } = await seedOrg("o01")
    const created = await createSession(table.id, "OrderGuest")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    const res = await fetch(`${API_URL}/sessions/${sessionId}/orders`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${guestToken}`,
      },
      body: JSON.stringify({
        idempotency_key: crypto.randomUUID(),
        items: [{ menu_item_id: menu.itemId, quantity: 2 }],
      }),
    })
    expect(res.status).toBe(201)
    // snake_case per openapi.yaml /sessions/{id}/orders. The response used to
    // leak the Go field names (Order/OrderItems) because PlaceOrderResult had
    // no JSON tags; this spec asserts the documented contract (F-24).
    const result = await res.json() as {
      order: { id: string; status: string; order_operational_id: string }
      order_items: Array<{ order_id: string }>
    }
    expect(result.order.id).toMatch(/^[0-9a-f-]{36}$/)
    expect(result.order.status).toBe("pending")
    expect(result.order.order_operational_id).toMatch(/^\S+$/)
    expect(result.order_items).toHaveLength(1)
    expect(result.order_items[0].order_id).toBe(result.order.id)
  })
})
