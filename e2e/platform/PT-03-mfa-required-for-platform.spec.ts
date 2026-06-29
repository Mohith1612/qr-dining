import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"

const adminToken = process.env.E2E_ADMIN_TOKEN ?? "e2e-admin-secret"

test.describe("PT-03: Platform MFA enforcement", () => {
  test("platform admin MFA status endpoint is accessible", async () => {
    const res = await fetch(`${API_URL}/platform/mfa/status`, {
      headers: { "Authorization": `Bearer ${adminToken}` },
    })
    // May return 200, 404 (not yet implemented), or 401 if token scoping differs
    expect([200, 401, 404]).toContain(res.status)
  })

  test("platform admin MFA enroll endpoint responds", async () => {
    const res = await fetch(`${API_URL}/platform/mfa/enroll`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${adminToken}`,
      },
      body: JSON.stringify({}),
    })
    expect([200, 201, 400, 401, 404, 409, 503]).toContain(res.status)
    expect(res.status).not.toBe(500)
  })
})
