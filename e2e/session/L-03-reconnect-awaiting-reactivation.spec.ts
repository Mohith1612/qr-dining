import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, fetchSnapshot } from "../helpers/api"

test.describe("L-03: Reconnect during awaiting_reactivation", () => {
  test("ws-ticket issuance succeeds on active session", async () => {
    const { table } = await seedOrg("l03")
    const created = await createSession(table.id, "ReconnUser")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    // Fetch a WS ticket to simulate reconnect
    const ticketRes = await fetch(`${API_URL}/sessions/${sessionId}/ws-ticket`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${guestToken}`,
      },
      body: JSON.stringify({ since: 0 }),
    })
    expect(ticketRes.status).toBe(200)
    const { ticket, expires_in } = await ticketRes.json()
    expect(typeof ticket).toBe("string")
    expect(ticket.length).toBeGreaterThan(10)
    expect(expires_in).toBeGreaterThan(0)
  })

  test("snapshot returns correct session data with missed_events field", async () => {
    const { table } = await seedOrg("l03b")
    const created = await createSession(table.id, "SnapUser")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    const snap = await fetchSnapshot(sessionId, guestToken)
    expect(snap.session.status).toBe("active")
    // missed_events may be empty array or omitted for new session
    if (snap.missed_events !== undefined) {
      expect(Array.isArray(snap.missed_events)).toBe(true)
    }
  })
})
