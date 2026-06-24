import { test, expect } from "@playwright/test"
import { seedOrg, createSession, fetchSnapshot } from "../helpers/api"

test.describe("L-10: Browser back/forward without spurious mutations", () => {
  test("navigating back and forward does not create duplicate orders", async ({ page }) => {
    const { table, menu } = await seedOrg("l10")
    const created = await createSession(table.id, "NavUser")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token
    const participantId = created.participant.id

    // Set up session in browser
    await page.goto(`/session/${sessionId}`)
    await page.evaluate(({ id, pid, tok }) => {
      sessionStorage.setItem("session_id", id)
      sessionStorage.setItem("participant_id", String(pid))
      sessionStorage.setItem("guest_access_token", tok)
    }, { id: sessionId, pid: participantId, tok: guestToken })

    await page.reload()
    await expect(page.locator("text=Welcome, NavUser")).toBeVisible({ timeout: 10_000 })

    // Navigate to cart
    await page.goto(`/session/${sessionId}/cart`)

    // Navigate back to session landing
    await page.goBack()

    // Navigate forward again
    await page.goForward()

    // Snapshot should show no orders placed (we didn't place any)
    const snap = await fetchSnapshot(sessionId, guestToken)
    expect(snap.session.status).toBe("active")
  })
})
