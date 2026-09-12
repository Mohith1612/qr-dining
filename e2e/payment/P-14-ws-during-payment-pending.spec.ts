import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, placeOrder } from "../helpers/api"
import { attemptTicketUpgrade } from "../helpers/websocket"
import crypto from "crypto"

// F-07: a guest who reconnects while their payment is pending used to be refused
// a WebSocket ticket (409 SESSION_CLOSED) and refused the upgrade, so they lost
// live bill and payment updates precisely during settlement and could only
// recover by reloading the page. payment_pending is non-terminal and is exactly
// when PAYMENT_COMPLETED / PAYMENT_CANCELLED arrive.

test.describe("P-14: WebSocket stays available during payment_pending", () => {
  test("ws-ticket is issued and the upgrade opens while a payment is pending", async ({ page }) => {
    const { table, menu } = await seedOrg("p14")
    const created = await createSession(table.id, "SettlingGuest")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    await placeOrder(sessionId, guestToken, menu.itemId)

    const payRes = await fetch(`${API_URL}/sessions/${sessionId}/payments`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${guestToken}`,
      },
      body: JSON.stringify({
        amount: menu.itemPrice,
        method: "cash",
        idempotency_key: crypto.randomUUID(),
      }),
    })
    expect([200, 201]).toContain(payRes.status)

    // Confirm the session really is frozen — otherwise this asserts nothing.
    const snapRes = await fetch(`${API_URL}/sessions/${sessionId}/snapshot`, {
      headers: { "Authorization": `Bearer ${guestToken}` },
    })
    expect(snapRes.status).toBe(200)
    const snap = await snapRes.json()
    expect(snap.session.status).toBe("payment_pending")

    // Reconnect: issue a ticket, then use it. Both gates are doubled in the
    // backend (ticket issuance and upgrade), so both must be exercised.
    const ticketRes = await fetch(`${API_URL}/sessions/${sessionId}/ws-ticket`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${guestToken}`,
      },
      body: JSON.stringify({ since: 0 }),
    })
    expect(ticketRes.status).toBe(201)
    const { ticket } = await ticketRes.json()

    await page.goto("/staff/login", { waitUntil: "domcontentloaded" })
    expect(await attemptTicketUpgrade(page, API_URL, ticket)).toBe("opened")
  })
})
