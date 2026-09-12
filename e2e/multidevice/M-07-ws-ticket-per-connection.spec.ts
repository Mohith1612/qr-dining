import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession } from "../helpers/api"

test.describe("M-07: WS ticket is per-connection — not shareable across devices", () => {
  // VACUOUS(sig-7): claims cross-device ticket behavior but performs no browser connection; passes when sharing succeeds.
  test.fixme("two concurrent ticket requests both succeed with distinct tickets", async () => {
    const { table } = await seedOrg("m07")
    const created = await createSession(table.id, "TicketUser")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    const fetchTicket = () =>
      fetch(`${API_URL}/sessions/${sessionId}/ws-ticket`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Authorization": `Bearer ${guestToken}`,
        },
        body: JSON.stringify({}),
      })

    const [r1, r2] = await Promise.all([fetchTicket(), fetchTicket()])

    expect([200, 201]).toContain(r1.status)
    // Second request may be rate-limited (429) or succeed with a different ticket
    expect([200, 201, 429]).toContain(r2.status)

    if (r1.ok && r2.ok) {
      const t1 = (await r1.json()).ticket
      const t2 = (await r2.json()).ticket
      // Tickets must be distinct one-shot tokens
      expect(t1).not.toEqual(t2)
    }
  })
})
