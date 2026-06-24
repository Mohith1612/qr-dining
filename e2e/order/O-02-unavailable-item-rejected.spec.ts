import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession } from "../helpers/api"
import crypto from "crypto"

const adminToken = process.env.E2E_ADMIN_TOKEN ?? "e2e-admin-secret"

test.describe("O-02: Ordering an unavailable item is rejected", () => {
  test("returns 400 or 422 when item is marked unavailable", async () => {
    const { table, menu, branch } = await seedOrg("o02")
    const created = await createSession(table.id, "OrderGuest")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    // Mark item unavailable
    await fetch(`${API_URL}/branches/${branch.id}/menu/items/${menu.itemId}`, {
      method: "PATCH",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${adminToken}`,
      },
      body: JSON.stringify({ is_available: false }),
    })

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
