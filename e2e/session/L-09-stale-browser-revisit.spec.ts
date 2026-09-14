import { test, expect } from "@playwright/test"
import { seedOrg, createSession, forceCloseSession } from "../helpers/api"

test.describe("L-09: Stale browser revisit after session closed", () => {
  test("UI shows session-ended screen when storage has closed session", async ({ page }) => {
    const { table } = await seedOrg("l09")
    const created = await createSession(table.id, "StaleTab")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token
    const participantId = created.participant.id

    // Close session on backend
    await forceCloseSession(sessionId, guestToken)

    // Simulate stale browser: navigate with old sessionStorage
    await page.goto(`/session/${sessionId}`)
    await page.evaluate(({ id, pid, tok }) => {
      sessionStorage.setItem("session_id", id)
      sessionStorage.setItem("participant_id", String(pid))
      sessionStorage.setItem("guest_access_token", tok)
    }, { id: sessionId, pid: participantId, tok: guestToken })

    await page.reload()

    // Should see session-ended screen (either "session has ended" or redirect to home)
    const ended = page.locator("text=/session has ended|ended|closed/i")

    // Wait for whichever recovery the app chooses. The second branch used to be
    // `page.waitForURL("/**")`, whose glob matches the URL the page is already
    // on — so the race resolved instantly, neither branch was ever awaited, and
    // the assertion below read whatever had happened to render in zero time.
    // That passed on a warm Chromium and failed on the first cold WebKit run.
    await Promise.race([
      ended.waitFor({ timeout: 10_000 }),
      page.waitForURL((url) => url.pathname === "/", { timeout: 10_000 }),
    ]).catch(() => {})

    const finalUrl = page.url()
    const isHome = finalUrl.endsWith("/") || finalUrl.endsWith("/table")
    const hasEndedText = await ended.isVisible().catch(() => false)

    expect(isHome || hasEndedText).toBe(true)
  })
})
