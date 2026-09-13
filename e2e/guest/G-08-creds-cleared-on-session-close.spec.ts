import { test, expect } from "@playwright/test"
import { seedOrg, createSession, forceCloseSession } from "../helpers/api"

// F-13: lib/guest-session.ts mirrored the guest bearer to
// localStorage["guest_creds:<session-uuid>"] and clearGuestCreds had zero
// callers, so a token valid for 12 hours outlived the meal on the device.
// Ending the session must forget it — from both localStorage and sessionStorage.

test.describe("G-08: guest credentials are cleared when the session ends", () => {
  test("closing the session removes the localStorage rejoin entry and the tab token", async ({ page }) => {
    const { table } = await seedOrg("g08")
    const created = await createSession(table.id, "ClosingGuest")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token
    const participantId = created.participant.id
    const lsKey = `guest_creds:${sessionId}`

    // Seed the credentials on the app origin before mounting the guarded session
    // route. Navigating to /session without them races SessionLayout's redirect
    // to /, so writing them afterwards does not prove SessionProvider mounted.
    await page.goto("/")
    await page.evaluate(({ id, pid, tok, key }) => {
      sessionStorage.setItem("session_id", id)
      sessionStorage.setItem("participant_id", String(pid))
      sessionStorage.setItem("guest_access_token", tok)
      localStorage.setItem(key, JSON.stringify({ participantId: pid, token: tok }))
    }, { id: sessionId, pid: participantId, tok: guestToken, key: lsKey })

    const snapshotPromise = page.waitForResponse((response) => {
      const url = new URL(response.url())
      return response.request().method() === "GET"
        && url.pathname === `/sessions/${sessionId}/snapshot`
    })
    await page.goto(`/session/${sessionId}`)

    // Precondition: the guarded route stayed live and SessionProvider completed
    // its initial snapshot, rather than merely finding a credential in storage.
    const snapshot = await snapshotPromise
    expect(snapshot.status()).toBe(200)
    await expect(page).toHaveURL(new RegExp(`/session/${sessionId}/?$`))

    await forceCloseSession(sessionId, guestToken)

    // The client learns the session ended (SESSION_CLOSED over the socket, or
    // the next snapshot) and must forget the credentials on that same path.
    await expect
      .poll(() => page.evaluate((key) => localStorage.getItem(key), lsKey), { timeout: 20_000 })
      .toBeNull()

    await expect
      .poll(() => page.evaluate(() => sessionStorage.getItem("guest_access_token")), { timeout: 10_000 })
      .toBeNull()
  })
})
