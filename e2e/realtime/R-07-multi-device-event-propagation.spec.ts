import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, fetchSnapshot } from "../helpers/api"
import crypto from "crypto"

test.describe("R-07: Multi-device event propagation", () => {
  // VACUOUS(sig-7): claims multi-device propagation but uses fetch only; passes when live device delivery is broken.
  test.fixme("order placed by participant A visible in snapshot for participant B", async () => {
    const { table, menu } = await seedOrg("r07")

    const hostCreated = await createSession(table.id, "DeviceA")
    const sessionId = hostCreated.session.id
    const tokenA = hostCreated.guest_access_token

    const joinRes = await fetch(`${API_URL}/sessions/${sessionId}/join`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ display_name: "DeviceB" }),
    })
    const joinData = await joinRes.json()
    const tokenB = joinData.guest_access_token

    // Device A places an order
    const orderRes = await fetch(`${API_URL}/sessions/${sessionId}/orders`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${tokenA}`,
      },
      body: JSON.stringify({
        idempotency_key: crypto.randomUUID(),
        items: [{ menu_item_id: menu.itemId, quantity: 1 }],
      }),
    })
    expect(orderRes.status).toBe(201)

    // Device B's snapshot should include the order event
    const snapB = await fetchSnapshot(sessionId, tokenB)
    expect(snapB.session.status).toBe("active")
    // The order is in server state — verified via orders endpoint
    const ordersRes = await fetch(`${API_URL}/sessions/${sessionId}/orders`, {
      headers: { "Authorization": `Bearer ${tokenB}` },
    })
    expect(ordersRes.status).toBe(200)
    const orders = await ordersRes.json()
    expect(Array.isArray(orders)).toBe(true)
    expect(orders.length).toBeGreaterThanOrEqual(1)
  })
})
