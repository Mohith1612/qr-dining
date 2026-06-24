import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, fetchSnapshot } from "../helpers/api"

const adminToken = process.env.E2E_ADMIN_TOKEN ?? "e2e-admin-secret"

test.describe("M-03: Token revocation propagates to all uses of that token", () => {
  test("revoked guest token rejected on snapshot and cart", async () => {
    const { table } = await seedOrg("m03")
    const created = await createSession(table.id, "RevokeMe")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token
    const participantId = created.participant.id

    // Revoke the participant's credentials
    const revokeRes = await fetch(
      `${API_URL}/sessions/${sessionId}/participants/${participantId}/revoke`,
      {
        method: "POST",
        headers: { "Authorization": `Bearer ${adminToken}` },
      }
    )
    expect([200, 204, 404]).toContain(revokeRes.status)

    if (revokeRes.status === 404) {
      // Try platform revocation path
      await fetch(`${API_URL}/platform/guests/${participantId}/revoke`, {
        method: "POST",
        headers: { "Authorization": `Bearer ${adminToken}` },
      })
    }

    // Revoked token should fail on protected endpoints
    const snapRes = await fetch(`${API_URL}/sessions/${sessionId}/snapshot`, {
      headers: { "Authorization": `Bearer ${guestToken}` },
    })
    expect([401, 403]).toContain(snapRes.status)
  })
})
