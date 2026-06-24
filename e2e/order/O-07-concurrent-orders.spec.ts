import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession } from "../helpers/api"
import crypto from "crypto"

test.describe("O-07: Concurrent orders from multiple guests", () => {
  test("two guests placing orders concurrently both succeed without corruption", async () => {
    const { table, menu } = await seedOrg("o07")
    const host = await createSession(table.id, "ConcurrentHost")
    const sessionId = host.session.id
    const hostToken = host.guest_access_token

    const joinRes = await fetch(`${API_URL}/sessions/${sessionId}/join`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ display_name: "ConcurrentGuest" }),
    })
    const guestToken = (await joinRes.json()).guest_access_token

    const placeOrder = (token: string) =>
      fetch(`${API_URL}/sessions/${sessionId}/orders`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Authorization": `Bearer ${token}`,
        },
        body: JSON.stringify({
          idempotency_key: crypto.randomUUID(),
          items: [{ menu_item_id: menu.itemId, quantity: 1 }],
        }),
      })

    const [r1, r2] = await Promise.all([
      placeOrder(hostToken),
      placeOrder(guestToken),
    ])

    expect([200, 201]).toContain(r1.status)
    expect([200, 201]).toContain(r2.status)

    // Each order should have a distinct ID
    const o1 = await r1.json()
    const o2 = await r2.json()
    expect(o1.id ?? o1.order_id).not.toEqual(o2.id ?? o2.order_id)
  })
})
