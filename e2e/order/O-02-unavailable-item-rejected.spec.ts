import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession } from "../helpers/api"
import crypto from "crypto"

test.describe("O-02: Ordering an unavailable item is rejected", () => {
  test("returns 400 or 422 when item is marked unavailable", async () => {
    const { table, menu, branch, owner } = await seedOrg("o02")
    const created = await createSession(table.id, "OrderGuest")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    // Mark item unavailable
    const unavailableRes = await fetch(`${API_URL}/menu/items/${menu.itemId}/availability`, {
      method: "PATCH",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${owner.token}`,
      },
      body: JSON.stringify({ available: false, branch_id: branch.id }),
    })
    expect(unavailableRes.status).toBe(204)

    const res = await fetch(`${API_URL}/sessions/${sessionId}/orders`, {
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
    expect([400, 409, 422]).toContain(res.status)
  })
})
