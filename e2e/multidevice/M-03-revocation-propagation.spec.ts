import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession } from "../helpers/api"

test.describe("M-03: Token revocation propagates to all uses of that token", () => {
  test("revoked guest token rejected on snapshot and cart", async () => {
    const { table, owner } = await seedOrg("m03")
    const created = await createSession(table.id, "RevokeMe")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    // Force-close is the registered operation that revokes every participant
    // credential for the session.
    const revokeRes = await fetch(`${API_URL}/sessions/${sessionId}/force-close`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${owner.token}`,
      },
      body: JSON.stringify({ reason: "e2e_revocation_propagation" }),
    })
    expect(revokeRes.status).toBe(200)

    // Revoked token should fail on protected endpoints
    const snapRes = await fetch(`${API_URL}/sessions/${sessionId}/snapshot`, {
      headers: { "Authorization": `Bearer ${guestToken}` },
    })
    expect(snapRes.status).toBe(401)

    const cartRes = await fetch(`${API_URL}/sessions/${sessionId}/cart`, {
      headers: { "Authorization": `Bearer ${guestToken}` },
    })
    expect(cartRes.status).toBe(401)
  })
})
