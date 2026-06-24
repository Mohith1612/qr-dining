import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession } from "../helpers/api"
import crypto from "crypto"

test.describe("O-05: Empty order rejected", () => {
  test("order with empty items array returns 400 or 422", async () => {
    const { table } = await seedOrg("o05")
    const created = await createSession(table.id, "EmptyOrderGuest")
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
        items: [],
      }),
    })
    expect([400, 422]).toContain(res.status)
  })

  test("order with zero quantity returns 400 or 422", async () => {
    const { table, menu } = await seedOrg("o05b")
    const created = await createSession(table.id, "ZeroQtyGuest")
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
        items: [{ menu_item_id: menu.itemId, quantity: 0 }],
      }),
    })
    expect([400, 422]).toContain(res.status)
  })
})
