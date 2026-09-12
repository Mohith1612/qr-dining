import { test, expect } from "@playwright/test"
import { BASE_URL } from "../playwright.config"

test.describe("F-04: Staff token in sessionStorage, not localStorage", () => {
  // VACUOUS(sig-5): never logs in; passes when the real login flow persists staff-auth in localStorage.
  test.fixme("staff-auth key is absent from localStorage after login", async ({ page }) => {
    await page.goto(`${BASE_URL}/staff/login`)

    // Verify page is accessible
    const title = await page.title()
    expect(title).toBeTruthy()

    // localStorage must not contain the staff auth token
    const localStaffAuth = await page.evaluate(() => localStorage.getItem("staff-auth"))
    expect(localStaffAuth).toBeNull()
  })
})
