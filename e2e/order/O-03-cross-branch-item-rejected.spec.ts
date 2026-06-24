import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession } from "../helpers/api"
import crypto from "crypto"

test.describe("O-03: Cross-branch item order rejected", () => {
  test("ordering item from a different branch returns 400/404", async () => {
    const orgA = await seedOrg("o03a")
    const orgB = await seedOrg("o03b")

    const sessionA = await createSession(orgA.table.id, "GuestA")
    const guestToken = sessionA.guest_access_token

    // Try to order orgB's item from orgA's session
    const res = await fetch(`${API_URL}/sessions/${sessionA.session.id}/orders`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${guestToken}`,
      },
      body: JSON.stringify({
        idempotency_key: crypto.randomUUID(),
        items: [{ menu_item_id: orgB.menu.itemId, quantity: 1 }],
      }),
    })
    expect([400, 403, 404, 422]).toContain(res.status)
  })
})
