import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession } from "../helpers/api"

test.describe("M-02: Concurrent cart mutations from two participants", () => {
  // VACUOUS(sig-3): accepts both mutation success and conflict; passes when shared-cart concurrency rejects a guest.
  test.fixme("two guests adding items concurrently both succeed or one gets a conflict", async () => {
    const { table, menu } = await seedOrg("m02")
    const created = await createSession(table.id, "GuestA")
    const sessionId = created.session.id
    const guestAToken = created.guest_access_token

    const joinRes = await fetch(`${API_URL}/sessions/${sessionId}/join`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ display_name: "GuestB" }),
    })
    expect(joinRes.ok).toBe(true)
    const guestBToken = (await joinRes.json()).guest_access_token

    const addItem = (token: string) =>
      fetch(`${API_URL}/sessions/${sessionId}/cart/items`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Authorization": `Bearer ${token}`,
        },
        body: JSON.stringify({ menu_item_id: menu.itemId, quantity: 1 }),
      })

    const [resA, resB] = await Promise.all([addItem(guestAToken), addItem(guestBToken)])

    // Both should succeed or one may get a conflict — no 500 errors
    expect([200, 201, 409]).toContain(resA.status)
    expect([200, 201, 409]).toContain(resB.status)

    // At least one must succeed
    const successCount = [resA.status, resB.status].filter((s) => s === 200 || s === 201).length
    expect(successCount).toBeGreaterThanOrEqual(1)
  })
})
