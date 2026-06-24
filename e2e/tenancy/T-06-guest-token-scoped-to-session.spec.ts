import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession } from "../helpers/api"

test.describe("T-06: Guest token scoped to one session only", () => {
  test("guest token from session A cannot access session B", async () => {
    const orgA = await seedOrg("t06a")
    const orgB = await seedOrg("t06b")

    const sessionA = await createSession(orgA.table.id, "GuestA")
    const sessionB = await createSession(orgB.table.id, "GuestB")

    const tokenA = sessionA.guest_access_token

    // Token from session A used against session B endpoints
    const snapRes = await fetch(`${API_URL}/sessions/${sessionB.session.id}/snapshot`, {
      headers: { "Authorization": `Bearer ${tokenA}` },
    })
    expect([401, 403, 404]).toContain(snapRes.status)

    const cartRes = await fetch(`${API_URL}/sessions/${sessionB.session.id}/cart/items`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${tokenA}`,
      },
      body: JSON.stringify({ menu_item_id: orgB.menu.itemId, quantity: 1 }),
    })
    expect([401, 403, 404]).toContain(cartRes.status)
  })
})
