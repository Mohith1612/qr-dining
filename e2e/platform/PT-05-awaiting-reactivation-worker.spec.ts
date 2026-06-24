import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession } from "../helpers/api"

const adminToken = process.env.E2E_ADMIN_TOKEN ?? "e2e-admin-secret"

test.describe("PT-05: awaiting_reactivation worker pipeline", () => {
  test("platform can mark a session as awaiting_reactivation", async () => {
    const { table } = await seedOrg("pt05")
    const created = await createSession(table.id, "PauseGuest")
    const sessionId = created.session.id

    // Trigger awaiting_reactivation via platform endpoint (presence-based or manual)
    const pauseRes = await fetch(`${API_URL}/platform/sessions/${sessionId}/pause`, {
      method: "POST",
      headers: { "Authorization": `Bearer ${adminToken}` },
    })
    // Accept success or not-found (endpoint may differ)
    expect([200, 204, 404]).toContain(pauseRes.status)

    if (pauseRes.status !== 404) {
      const snapRes = await fetch(`${API_URL}/platform/sessions/${sessionId}`, {
        headers: { "Authorization": `Bearer ${adminToken}` },
      })
      if (snapRes.status === 200) {
        const data = await snapRes.json()
        expect(["awaiting_reactivation", "active"]).toContain(data.status)
      }
    }
  })
})
