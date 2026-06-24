import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, fetchSnapshot } from "../helpers/api"

test.describe("M-06: Host close — snapshot reflects closed state for all participants", () => {
  test("after host closes session, guest snapshot returns closed status", async () => {
    const { table } = await seedOrg("m06")
    const host = await createSession(table.id, "Host")
    const sessionId = host.session.id
    const hostToken = host.guest_access_token

    const joinRes = await fetch(`${API_URL}/sessions/${sessionId}/join`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ display_name: "GuestB" }),
    })
    const guestBToken = (await joinRes.json()).guest_access_token

    // Host closes the session
    const closeRes = await fetch(`${API_URL}/sessions/${sessionId}`, {
      method: "DELETE",
      headers: { "Authorization": `Bearer ${hostToken}` },
    })
    expect([200, 204]).toContain(closeRes.status)

    // Guest B snapshot should reflect closed
    const snapRes = await fetch(`${API_URL}/sessions/${sessionId}/snapshot`, {
      headers: { "Authorization": `Bearer ${guestBToken}` },
    })
    // May return 404 (session gone) or 200 with closed status
    if (snapRes.status === 200) {
      const snap = await snapRes.json()
      expect(["closed", "abandoned", "expired"]).toContain(snap.session.status)
    } else {
      expect([401, 403, 404, 410]).toContain(snapRes.status)
    }
  })
})
