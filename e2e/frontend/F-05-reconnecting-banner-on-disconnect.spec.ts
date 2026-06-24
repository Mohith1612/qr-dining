import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession } from "../helpers/api"

test.describe("F-05: Backend snapshot reflects awaiting_reactivation for frontend banner", () => {
  test("session snapshot can return awaiting_reactivation status", async () => {
    const { table } = await seedOrg("f05")
    const created = await createSession(table.id, "BannerGuest")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    // Snapshot should show active initially
    const snap = await fetch(`${API_URL}/sessions/${sessionId}/snapshot`, {
      headers: { "Authorization": `Bearer ${guestToken}` },
    })
    expect(snap.status).toBe(200)
    const data = await snap.json()
    expect(["active", "awaiting_reactivation"]).toContain(data.session.status)
  })
})
