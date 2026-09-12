import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession } from "../helpers/api"
import { attemptTicketUpgrade } from "../helpers/websocket"

test.describe("X-07: WS ticket replay attack — expired/used ticket rejected", () => {
  test("ticket expires within 30 seconds and cannot be reused", async ({ page }) => {
    const { table } = await seedOrg("x07")
    const created = await createSession(table.id, "ReplayUser")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    // Get a ticket
    const ticketRes = await fetch(`${API_URL}/sessions/${sessionId}/ws-ticket`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${guestToken}`,
      },
      body: JSON.stringify({}),
    })
    expect(ticketRes.status).toBe(201)
    const { ticket, expires_in } = await ticketRes.json()
    expect(expires_in).toBe(30)
    expect(typeof ticket).toBe("string")
    expect(ticket.length).toBeGreaterThan(10)

    // A real browser upgrade consumes the Redis ticket. Replaying the exact
    // same ticket must fail to open a second connection.
    await page.goto("/staff/login", { waitUntil: "domcontentloaded" })
    expect(await attemptTicketUpgrade(page, API_URL, ticket)).toBe("opened")
    expect(await attemptTicketUpgrade(page, API_URL, ticket)).toBe("rejected")
  })
})
