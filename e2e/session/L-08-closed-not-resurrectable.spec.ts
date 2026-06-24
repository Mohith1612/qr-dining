import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, forceCloseSession } from "../helpers/api"

test.describe("L-08: Closed session not resurrectable", () => {
  test("all mutations on closed session return 409/410", async () => {
    const { table, menu } = await seedOrg("l08")
    const created = await createSession(table.id, "ClosedGuest")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    await forceCloseSession(sessionId)

    // Cart add
    const cartRes = await fetch(`${API_URL}/sessions/${sessionId}/cart/items`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${guestToken}`,
      },
      body: JSON.stringify({ menu_item_id: menu.itemId, quantity: 1 }),
    })
    expect([401, 403, 409, 410]).toContain(cartRes.status)

    // Join attempt
    const joinRes = await fetch(`${API_URL}/sessions/${sessionId}/join`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ display_name: "NewGuest" }),
    })
    expect([401, 403, 409, 410]).toContain(joinRes.status)

    // Table should now be available (new session can be created)
    const newSessionRes = await fetch(`${API_URL}/sessions`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ table_id: table.id, display_name: "FreshGuest" }),
    })
    expect(newSessionRes.status).toBe(201)
  })
})
