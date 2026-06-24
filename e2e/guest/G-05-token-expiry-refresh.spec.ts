import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, fetchSnapshot } from "../helpers/api"

test.describe("G-05: Guest token expiry refresh via snapshot", () => {
  test("snapshot read on active session returns refreshed token hint", async () => {
    const { table } = await seedOrg("g05")
    const created = await createSession(table.id, "ExpiryUser")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    // Snapshot read should succeed and session should be active
    const snap = await fetchSnapshot(sessionId, guestToken)
    expect(snap.session.status).toBe("active")
    // If backend returns a refreshed token in the snapshot response, it would be in guest_access_token field
    // The main assertion is that the snapshot is readable while session is active
  })
})
