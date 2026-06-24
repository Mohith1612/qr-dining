import { test, expect } from "@playwright/test"
import { seedOrg, createSession, fetchSnapshot } from "../helpers/api"

test.describe("R-08: Presence expiry — session structure", () => {
  test("session snapshot includes expected fields for presence-based UX", async () => {
    const { table } = await seedOrg("r08")
    const created = await createSession(table.id, "PresenceUser")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    const snap = await fetchSnapshot(sessionId, guestToken)

    // Presence fields expected in participants
    expect(snap.participants.length).toBeGreaterThanOrEqual(1)
    const self = snap.participants.find((p) => p.display_name === "PresenceUser")
    expect(self).toBeDefined()
    // is_host expected
    expect(typeof self?.is_host).toBe("boolean")
  })
})
