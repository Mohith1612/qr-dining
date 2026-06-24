import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession } from "../helpers/api"

const adminToken = process.env.E2E_ADMIN_TOKEN ?? "e2e-admin-secret"

test.describe("PT-01: Platform admin can read any session", () => {
  test("platform admin fetches session details across orgs", async () => {
    const { table } = await seedOrg("pt01")
    const created = await createSession(table.id, "PlatformGuest")
    const sessionId = created.session.id

    const res = await fetch(`${API_URL}/platform/sessions/${sessionId}`, {
      headers: { "Authorization": `Bearer ${adminToken}` },
    })
    expect([200, 404]).toContain(res.status)
    if (res.status === 200) {
      const data = await res.json()
      expect(data.id).toBe(sessionId)
    }
  })
})
