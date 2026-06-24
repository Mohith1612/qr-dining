import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, loginStaff } from "../helpers/api"

const adminToken = process.env.E2E_ADMIN_TOKEN ?? "e2e-admin-secret"

test.describe("S-08: Owner cannot access platform admin endpoints", () => {
  test("owner token rejected on platform-level org management", async () => {
    const { branch, org } = await seedOrg("s08")

    // Seed an owner-role staff
    const ownerCode = `owner-${Date.now()}`
    await fetch(`${API_URL}/branches/${branch.id}/staff`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${adminToken}`,
      },
      body: JSON.stringify({
        name: "Branch Owner",
        role: "owner",
        staff_code: ownerCode,
        pin: "777777",
      }),
    })

    const ownerCtx = await loginStaff(branch.code, ownerCode, "777777")

    // Owner tries to create a new organization — platform-only operation
    const res = await fetch(`${API_URL}/platform/organizations`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${ownerCtx.token}`,
      },
      body: JSON.stringify({ name: "Unauthorized Org", slug: `unauth-${Date.now()}` }),
    })
    expect([401, 403]).toContain(res.status)
  })
})
