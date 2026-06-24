import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, forceCloseSession, fetchSnapshot } from "../helpers/api"

test.describe("PT-02: Platform admin can force-close any session", () => {
  test("force-close sets session status to closed", async () => {
    const { table } = await seedOrg("pt02")
    const created = await createSession(table.id, "ForceCloseGuest")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    await forceCloseSession(sessionId)

    const snapRes = await fetch(`${API_URL}/sessions/${sessionId}/snapshot`, {
      headers: { "Authorization": `Bearer ${guestToken}` },
    })

    if (snapRes.status === 200) {
      const snap = await snapRes.json()
      expect(["closed", "abandoned", "expired"]).toContain(snap.session.status)
    } else {
      expect([401, 403, 404, 410]).toContain(snapRes.status)
    }
  })
})
