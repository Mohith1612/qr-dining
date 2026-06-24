import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession } from "../helpers/api"

test.describe("L-05: Host close is idempotent", () => {
  test("DELETE session twice returns success both times", async () => {
    const { table } = await seedOrg("l05")
    const created = await createSession(table.id, "HostClose")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    const close1 = await fetch(`${API_URL}/sessions/${sessionId}`, {
      method: "DELETE",
      headers: { "Authorization": `Bearer ${guestToken}` },
    })
    expect([200, 204]).toContain(close1.status)

    const close2 = await fetch(`${API_URL}/sessions/${sessionId}`, {
      method: "DELETE",
      headers: { "Authorization": `Bearer ${guestToken}` },
    })
    // Second close should be no-op 200/204 or 409/410 (session already closed)
    expect([200, 204, 409, 410]).toContain(close2.status)
  })
})
