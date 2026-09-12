import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession } from "../helpers/api"

test.describe("L-05: Host close revokes the guest credential", () => {
  test("first close succeeds and replay with the revoked credential is rejected", async () => {
    const { table } = await seedOrg("l05")
    const created = await createSession(table.id, "HostClose")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    const close1 = await fetch(`${API_URL}/sessions/${sessionId}`, {
      method: "DELETE",
      headers: { "Authorization": `Bearer ${guestToken}` },
    })
    expect(close1.status).toBe(204)

    const close2 = await fetch(`${API_URL}/sessions/${sessionId}`, {
      method: "DELETE",
      headers: { "Authorization": `Bearer ${guestToken}` },
    })
    expect(close2.status).toBe(401)
    const replayError = await close2.json()
    expect(replayError.code).toBe("UNAUTHORIZED")
  })
})
