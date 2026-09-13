import { test, expect } from "@playwright/test"
import { seedOrg, createSession, placeOrder, fetchAudit, fetchSnapshot } from "../helpers/api"

test.describe("G-01: Fresh guest session and order", () => {
  test("guest scans QR, sets name, adds item, places order", async ({ page }) => {
    const { branch, table, menu } = await seedOrg("g01")

    const created = await createSession(table.id, "Alice")
    const { sessionId, guestToken } = { sessionId: created.session.id, guestToken: created.guest_access_token }

    // Inject credentials on the app origin before mounting the guarded session
    // route, matching the ordering of the real QR flow.
    await page.goto("/")
    await page.evaluate(({ id, pid, tok }) => {
      sessionStorage.setItem("session_id", id)
      sessionStorage.setItem("participant_id", String(pid))
      sessionStorage.setItem("guest_access_token", tok)
    }, { id: sessionId, pid: created.participant.id, tok: guestToken })

    const snapshotPromise = page.waitForResponse((response) => {
      const url = new URL(response.url())
      return response.request().method() === "GET"
        && url.pathname === `/sessions/${sessionId}/snapshot`
    })
    await page.goto(`/session/${sessionId}`)

    const initialSnapshot = await snapshotPromise
    expect(initialSnapshot.status()).toBe(200)
    await expect(page).toHaveURL(new RegExp(`/session/${sessionId}/?$`))
    await expect(page.locator("text=Welcome, Alice")).toBeVisible({ timeout: 10_000 })

    // Navigate to menu and add an item
    await page.goto(`/session/${sessionId}/menu`)
    await expect(page.locator(`text=${menu.itemName}`).first()).toBeVisible({ timeout: 8_000 })

    // Navigate to cart and place order
    await placeOrder(sessionId, guestToken, menu.itemId)

    // Verify via API
    const snap = await fetchSnapshot(sessionId, guestToken)
    expect(snap.session.status).toBe("active")

    const audit = await fetchAudit("session", sessionId)
    const createEvent = audit.find((a) => a.action === "session.create")
    expect(createEvent).toBeDefined()
  })
})
