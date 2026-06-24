import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession } from "../helpers/api"

test.describe("T-02: Cross-branch menu item isolation", () => {
  test("guest cannot order a menu item from a different branch", async () => {
    const orgA = await seedOrg("t02a")
    const orgB = await seedOrg("t02b")

    // Guest on branch A tries to order Branch B's menu item
    const sessionA = await createSession(orgA.table.id, "GuestA")
    const guestToken = sessionA.guest_access_token

    const res = await fetch(`${API_URL}/sessions/${sessionA.session.id}/orders`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${guestToken}`,
      },
      body: JSON.stringify({
        idempotency_key: `t02-cross-${Date.now()}`,
        items: [{ menu_item_id: orgB.menu.itemId, quantity: 1 }],
      }),
    })
    expect([400, 403, 404, 422]).toContain(res.status)
  })
})
