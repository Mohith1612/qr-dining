import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, fetchSnapshot } from "../helpers/api"

test.describe("M-04: Second device joins existing session", () => {
  // VACUOUS(sig-7): claims second-device behavior but uses fetch only; passes when device rejoin UI is broken.
  test.fixme("joining guest sees existing session state in snapshot", async () => {
    const { table, menu } = await seedOrg("m04")
    const host = await createSession(table.id, "Host")
    const sessionId = host.session.id
    const hostToken = host.guest_access_token

    // Host places an order
    await fetch(`${API_URL}/sessions/${sessionId}/orders`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${hostToken}`,
      },
      body: JSON.stringify({
        idempotency_key: `m04-order-${Date.now()}`,
        items: [{ menu_item_id: menu.itemId, quantity: 1 }],
      }),
    })

    // New device joins
    const joinRes = await fetch(`${API_URL}/sessions/${sessionId}/join`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ display_name: "NewDevice" }),
    })
    expect(joinRes.status).toBe(201)
    const { guest_access_token: newToken } = await joinRes.json()

    // New device fetches snapshot — should see the session
    const snap = await fetchSnapshot(sessionId, newToken)
    expect(snap.session.status).toBe("active")
    expect(snap.participants.length).toBeGreaterThanOrEqual(2)
  })
})
