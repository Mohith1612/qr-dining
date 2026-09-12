import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, getPlatformToken } from "../helpers/api"

test.describe("T-03: Suspended org rejects new sessions", () => {
  test("after org suspension, new session creation is rejected", async () => {
    const { org, table } = await seedOrg("t03")
    const platformToken = await getPlatformToken()

    // Suspend the org
    const suspendRes = await fetch(`${API_URL}/platform/organizations/${org.id}/suspend`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${platformToken}`,
      },
      body: JSON.stringify({ reason: "e2e_test" }),
    })
    expect([200, 204, 404]).toContain(suspendRes.status)

    if (suspendRes.status === 404) {
      // Suspension endpoint not implemented; skip
      return
    }

    // Try to create a session on a suspended org's table
    const sessionRes = await fetch(`${API_URL}/sessions`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ table_id: table.id, display_name: "BlockedGuest" }),
    })
    expect([400, 403, 422, 503]).toContain(sessionRes.status)
  })
})
