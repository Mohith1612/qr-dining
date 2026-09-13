import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, getPlatformToken } from "../helpers/api"

test.describe("PT-01: Platform admin can read any session", () => {
  test("platform admin fetches session details across orgs", async () => {
    const { table } = await seedOrg("pt01")
    const created = await createSession(table.id, "PlatformGuest")
    const sessionId = created.session.id
    const platformToken = await getPlatformToken()

    const res = await fetch(`${API_URL}/platform/sessions/${sessionId}`, {
      headers: { "Authorization": `Bearer ${platformToken}` },
    })
    expect(res.status).toBe(200)
    const data = await res.json()
    expect(data.session.id).toBe(sessionId)
  })
})
