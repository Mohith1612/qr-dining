import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession } from "../helpers/api"

test.describe("F-01: QR rescan joins existing active session", () => {
  // VACUOUS(sig-7): claims a QR rescan but uses fetch only; passes when the browser rescan flow is broken.
  test.fixme("scanning same table QR again after session exists allows join", async () => {
    const { table } = await seedOrg("f01")

    // First scan creates session
    await createSession(table.id, "FirstGuest")

    // Second scan should join or start another session — not fail with 500
    const joinRes = await fetch(`${API_URL}/sessions`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ table_id: table.id, display_name: "SecondGuest" }),
    })
    // Should either join existing or return conflict with session ID to join
    expect([200, 201, 409]).toContain(joinRes.status)
    expect(joinRes.status).not.toBe(500)

    if (joinRes.status === 409) {
      const err = await joinRes.json()
      // Conflict response should include the existing session ID to join
      expect(err.session_id ?? err.existing_session_id ?? err.code).toBeTruthy()
    }
  })
})
