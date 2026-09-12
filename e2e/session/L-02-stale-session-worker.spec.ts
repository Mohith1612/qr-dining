import { test, expect } from "@playwright/test"
import { seedOrg, createSession, fetchSnapshot, fetchSessionDB } from "../helpers/api"

test.describe("L-02: Stale session worker abandons and releases table", () => {
  // VACUOUS(sig-6): fixture never ages the session or runs the stale worker; passes when cleanup never occurs.
  test.fixme("session with no activity eventually reaches terminal state", async () => {
    const { table } = await seedOrg("l02")
    const created = await createSession(table.id, "StaleUser")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    // Verify session is initially active
    const snap = await fetchSnapshot(sessionId, guestToken)
    expect(snap.session.status).toBe("active")

    // Note: We cannot wait out the full grace period in a test.
    // This test verifies the API contract exists — actual timing verification
    // is done in staging burn-in scenario S2.
    // Here we verify the snapshot endpoint and session structure are correct.
    expect(snap.session.id).toBe(sessionId)
    expect(typeof snap.session.branch_id).toBe("number")
  })
})
