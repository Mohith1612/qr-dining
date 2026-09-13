import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession } from "../helpers/api"
import crypto from "crypto"

test.describe("O-07: Host-only order placement", () => {
  test("non-host order is rejected and the host order succeeds", async () => {
    const { table, menu } = await seedOrg("o07")
    const host = await createSession(table.id, "ConcurrentHost")
    const sessionId = host.session.id
    const hostToken = host.guest_access_token

    const joinRes = await fetch(`${API_URL}/sessions/${sessionId}/join`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ display_name: "ConcurrentGuest" }),
    })
    expect(joinRes.status).toBe(201)
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

    const nonHostOrder = await placeOrder(guestToken)
    expect(nonHostOrder.status).toBe(403)
    const denial = await nonHostOrder.json()
    expect(denial.code).toBe("NOT_SESSION_HOST")

    const hostOrder = await placeOrder(hostToken)
    expect(hostOrder.status).toBe(201)
    const result = await hostOrder.json() as {
      order: { id: string; status: string; placed_by_participant_id: number }
    }
    expect(result.order.id).toMatch(/^[0-9a-f-]{36}$/)
    expect(result.order.status).toBe("pending")
    expect(result.order.placed_by_participant_id).toBe(host.participant.id)
  })
})
