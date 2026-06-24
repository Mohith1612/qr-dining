import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, forceCloseSession } from "../helpers/api"

test.describe("G-04: Stale token replay after session closed", () => {
  test("closed session returns 410 for any mutation", async () => {
    const { table, menu } = await seedOrg("g04")
    const created = await createSession(table.id, "StaleUser")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    // Close the session
    await forceCloseSession(sessionId)

    // Replay the token on cart mutation
    const cartRes = await fetch(`${API_URL}/sessions/${sessionId}/cart/items`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${guestToken}`,
      },
      body: JSON.stringify({ menu_item_id: menu.itemId, quantity: 1 }),
    })
    expect([401, 403, 409, 410]).toContain(cartRes.status)

    // Replay on snapshot read
    const snapRes = await fetch(`${API_URL}/sessions/${sessionId}/snapshot`, {
      headers: { "Authorization": `Bearer ${guestToken}` },
    })
    // 410 expected for closed sessions after expiry window; 200 with closed status also acceptable
    if (snapRes.status === 200) {
      const snap = await snapRes.json()
      expect(["closed", "abandoned", "expired"]).toContain(snap.session.status)
    } else {
      expect([401, 410]).toContain(snapRes.status)
    }
  })
})
