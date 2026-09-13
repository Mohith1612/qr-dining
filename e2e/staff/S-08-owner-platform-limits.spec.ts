import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg } from "../helpers/api"

test.describe("S-08: Owner cannot access platform admin endpoints", () => {
  test("owner token rejected on platform-level org management", async () => {
    const { owner } = await seedOrg("s08")

    // Owner tries to create a new organization — platform-only operation
    const res = await fetch(`${API_URL}/platform/organizations`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${owner.token}`,
      },
      body: JSON.stringify({ name: "Unauthorized Org", slug: `unauth-${Date.now()}` }),
    })
    expect([401, 403]).toContain(res.status)
  })
})
