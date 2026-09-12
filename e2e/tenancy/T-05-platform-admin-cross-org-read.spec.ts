import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, getPlatformToken } from "../helpers/api"

test.describe("T-05: Platform admin can read across all orgs", () => {
  // VACUOUS(sig-5): never creates or uses org A staff; passes when tenant staff can also read org B.
  test.fixme("platform admin reads org B session that org A staff cannot", async () => {
    const orgB = await seedOrg("t05b")
    const sessionB = await createSession(orgB.table.id, "T05Guest")
    const sessionId = sessionB.session.id
    const platformToken = await getPlatformToken()

    // Platform admin can read it
    const res = await fetch(`${API_URL}/platform/sessions/${sessionId}`, {
      headers: { "Authorization": `Bearer ${platformToken}` },
    })
    expect(res.status).toBe(200)
    const data = await res.json()
    expect(data.session.id).toBe(sessionId)
  })
})
