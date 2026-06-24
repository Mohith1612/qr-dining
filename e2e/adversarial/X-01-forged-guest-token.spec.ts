import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession } from "../helpers/api"
import crypto from "crypto"

test.describe("X-01: Forged guest token rejected", () => {
  test("tampered signature on guest token returns 401", async () => {
    const { table, menu } = await seedOrg("x01")
    const created = await createSession(table.id, "LegitUser")
    const { id: sessionId } = created.session
    const validToken = created.guest_access_token

    // Tamper: replace the signature portion with garbage
    const parts = validToken.split(".")
    expect(parts.length).toBeGreaterThanOrEqual(2)
    const tamperedToken = parts[0] + "." + Buffer.from(crypto.randomBytes(32)).toString("base64url")

    const res = await fetch(`${API_URL}/sessions/${sessionId}/cart`, {
      headers: { "Authorization": `Bearer ${tamperedToken}` },
    })
    expect(res.status).toBe(401)
  })

  test("token with wrong session_id rejected on different session", async () => {
    const { table } = await seedOrg("x01b")
    const created1 = await createSession(table.id, "UserA")
    const token1 = created1.guest_access_token

    // Create a second table and session
    const { table: table2 } = await seedOrg("x01c")
    const created2 = await createSession(table2.id, "UserB")
    const session2Id = created2.session.id

    // Use token from session 1 against session 2
    const res = await fetch(`${API_URL}/sessions/${session2Id}/cart`, {
      headers: { "Authorization": `Bearer ${token1}` },
    })
    expect([401, 403]).toContain(res.status)
  })
})
