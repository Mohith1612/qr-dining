import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, forceCloseSession } from "../helpers/api"

test.describe("L-04: Reconnect after session abandoned", () => {
  test("force-closed session returns 410 or closed status for snapshot", async () => {
    const { table } = await seedOrg("l04")
    const created = await createSession(table.id, "AbandonUser")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    await forceCloseSession(sessionId, guestToken)

    // WS ticket should be rejected for closed session
    const ticketRes = await fetch(`${API_URL}/sessions/${sessionId}/ws-ticket`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${guestToken}`,
      },
      body: JSON.stringify({}),
    })
    expect([401, 403, 409, 410]).toContain(ticketRes.status)

    // Snapshot should reflect terminal state
    const snapRes = await fetch(`${API_URL}/sessions/${sessionId}/snapshot`, {
      headers: { "Authorization": `Bearer ${guestToken}` },
    })
    if (snapRes.status === 200) {
      const snap = await snapRes.json()
      expect(["closed", "abandoned", "expired"]).toContain(snap.session.status)
    } else {
      expect([401, 410]).toContain(snapRes.status)
    }
  })
})
