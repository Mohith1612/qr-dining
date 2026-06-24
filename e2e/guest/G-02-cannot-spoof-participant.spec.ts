import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession } from "../helpers/api"

test.describe("G-02: Guest cannot spoof another participant", () => {
  test("order attributed to token identity, not body participant_id", async () => {
    const { table, menu } = await seedOrg("g02")

    // Two guests join the same session
    const host = await createSession(table.id, "Host")
    const sessionId = host.session.id

    const guestRes = await fetch(`${API_URL}/sessions/${sessionId}/join`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ display_name: "Guest" }),
    })
    const guestData = await guestRes.json()
    const guestToken = guestData.guest_access_token
    const hostParticipantId = host.participant.id

    // Guest tries to spoof host's participant_id in request body
    const orderRes = await fetch(`${API_URL}/sessions/${sessionId}/orders`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${guestToken}`,
      },
      body: JSON.stringify({
        idempotency_key: crypto.randomUUID(),
        items: [{ menu_item_id: menu.itemId, quantity: 1 }],
        // Legacy attempt to spoof a different participant
        participant_id: hostParticipantId,
      }),
    })

    // Backend should either reject (if strict) or attribute to token's participant
    if (orderRes.status === 201) {
      const order = await orderRes.json()
      // If accepted, the order must be attributed to the guest's participant (from token), not host's
      expect(order.Order?.participant_id).not.toBe(hostParticipantId)
    } else {
      // 401 or 403 when AUTH_GUEST_CREDENTIALS_REQUIRED is enabled
      expect([401, 403]).toContain(orderRes.status)
    }
  })
})
