import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, placeOrder, fetchSnapshot } from "../helpers/api"

test.describe("M-01: Dual-tab — cart updates visible in both tabs via snapshot", () => {
  // VACUOUS(sig-7): claims two-tab propagation but uses API helpers only; passes when tab synchronization is broken.
  test.fixme("order placed from tab A appears in snapshot fetched by tab B (same guest token)", async () => {
    const { table, menu } = await seedOrg("m01")
    const created = await createSession(table.id, "DualTab")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    // Tab A places an order
    await placeOrder(sessionId, guestToken, menu.itemId, 1)

    // Tab B fetches snapshot using same token
    const snap = await fetchSnapshot(sessionId, guestToken)
    expect(snap.session).toBeDefined()

    // The session should be active (not closed)
    expect(snap.session.status).toBe("active")
  })
})
