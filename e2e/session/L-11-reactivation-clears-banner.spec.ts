import { test, expect } from "@playwright/test"
import { seedOrg, createSession } from "../helpers/api"

// F-08: markPaused raises the full-width "table is paused" overlay from the
// reconnect path, but nothing on the AUTOMATIC reactivation path lowered it.
// The only handlers that cleared isReactivating were a dead SESSION_REACTIVATED
// handler (the backend published SESSION_CREATED instead) and the manual
// "We're still ordering" button. setFromSnapshot — the function that actually
// runs when the server reactivates the session — did not. So the overlay sat,
// counting down to 0:00, over a live and connected session.
//
// awaiting_reactivation is produced by the presence worker on a timer, which is
// not reachable inside a test's budget, so the server states are staged with
// route interception. Everything under test is client-side: the paused overlay
// is raised by the client's own reconnect path and must be lowered by the
// client's own snapshot reconcile.

test.describe("L-11: automatic reactivation clears the paused banner", () => {
  test("the paused overlay disappears once the snapshot reports the session active again", async ({ page }) => {
    const { table } = await seedOrg("l11")
    const created = await createSession(table.id, "PausedGuest")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token
    const participantId = created.participant.id

    // Stage 1: the socket cannot be established and the session reads as paused,
    // which is what drives the client into markPaused().
    let paused = true
    await page.route("**/sessions/*/ws-ticket", (route) => route.abort())
    await page.route("**/sessions/*/snapshot*", async (route) => {
      const response = await route.fetch()
      const body = await response.json()
      if (paused) body.session.status = "awaiting_reactivation"
      await route.fulfill({ response, json: body })
    })

    // Establish the browser identity before mounting the guarded session route.
    // Otherwise SessionLayout can redirect to / before these writes land.
    await page.goto("/")
    await page.evaluate(({ id, pid, tok }) => {
      sessionStorage.setItem("session_id", id)
      sessionStorage.setItem("participant_id", String(pid))
      sessionStorage.setItem("guest_access_token", tok)
    }, { id: sessionId, pid: participantId, tok: guestToken })

    const snapshotPromise = page.waitForResponse((response) => {
      const url = new URL(response.url())
      return response.request().method() === "GET"
        && url.pathname === `/sessions/${sessionId}/snapshot`
    })
    await page.goto(`/session/${sessionId}`)

    const initialSnapshot = await snapshotPromise
    expect(initialSnapshot.status()).toBe(200)
    await expect(page).toHaveURL(new RegExp(`/session/${sessionId}/?$`))

    const pausedBanner = page.getByText(/table is paused/i)
    await expect(pausedBanner).toBeVisible({ timeout: 12_000 })

    // Stage 2: the server reactivates the session (this is what the real
    // snapshot endpoint does on reconnect). The overlay must come down without
    // the guest touching anything — no manual button, no page reload.
    paused = false

    await expect(pausedBanner).toBeHidden({ timeout: 12_000 })
  })
})
