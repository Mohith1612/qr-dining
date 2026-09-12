import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession } from "../helpers/api"
import { attemptTicketUpgrade } from "../helpers/websocket"

test.describe("R-01: WebSocket ticket authentication", () => {
  test("ws-ticket endpoint requires valid guest token", async () => {
    const { table } = await seedOrg("r01")
    const created = await createSession(table.id, "WSUser")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    // Valid token → should succeed
    const validRes = await fetch(`${API_URL}/sessions/${sessionId}/ws-ticket`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${guestToken}`,
      },
      body: JSON.stringify({ since: 0 }),
    })
    expect(validRes.status).toBe(201)
    const { ticket } = await validRes.json()
    expect(typeof ticket).toBe("string")

    // No token → 401
    const noAuthRes = await fetch(`${API_URL}/sessions/${sessionId}/ws-ticket`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({}),
    })
    expect(noAuthRes.status).toBe(401)

    // Invalid token → 401
    const badAuthRes = await fetch(`${API_URL}/sessions/${sessionId}/ws-ticket`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": "Bearer invalid.token.here",
      },
      body: JSON.stringify({}),
    })
    expect(badAuthRes.status).toBe(401)
  })

  test("ticket is single-use — second upgrade attempt fails", async ({ page }) => {
    const { table } = await seedOrg("r01b")
    const created = await createSession(table.id, "SingleUseUser")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    const ticketRes = await fetch(`${API_URL}/sessions/${sessionId}/ws-ticket`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${guestToken}`,
      },
      body: JSON.stringify({}),
    })
    expect(ticketRes.status).toBe(201)
    const { ticket } = await ticketRes.json()
    expect(typeof ticket).toBe("string")
    expect(ticket.length).toBeGreaterThan(10)

    await page.goto("/staff/login", { waitUntil: "domcontentloaded" })
    expect(await attemptTicketUpgrade(page, API_URL, ticket)).toBe("opened")
    expect(await attemptTicketUpgrade(page, API_URL, ticket)).toBe("rejected")
  })
})
