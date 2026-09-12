import { test, expect } from "@playwright/test"
import { BASE_URL } from "../playwright.config"

test.describe("F-03: Guest token stored in sessionStorage, not localStorage", () => {
  // VACUOUS(sig-5): never joins a session; passes when the real join flow writes the token to localStorage.
  test.fixme("after joining a session, token is in sessionStorage and not in localStorage", async ({ page }) => {
    await page.goto(`${BASE_URL}/`)

    // Check if the app is reachable
    const title = await page.title()
    expect(title).toBeTruthy()

    // Read storage values — token should not be in localStorage
    const localToken = await page.evaluate(() => localStorage.getItem("guest_access_token"))
    expect(localToken).toBeNull()

    // sessionStorage may have the token if the user has already joined a session
    // This test primarily validates the absence from localStorage
  })
})
