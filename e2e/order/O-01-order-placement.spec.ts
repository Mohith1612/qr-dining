import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession } from "../helpers/api"
import crypto from "crypto"

test.describe("O-01: Order placement — happy path", () => {
  test("guest places order and receives confirmed order with operational ID", async () => {
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
    expect([200, 201]).toContain(res.status)
    const order = await res.json()
    expect(order.id ?? order.order_id).toBeTruthy()
    expect(order.status ?? order.order_status).toBeDefined()
  })
})
