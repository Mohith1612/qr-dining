import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg } from "../helpers/api"

const adminToken = process.env.E2E_ADMIN_TOKEN ?? "e2e-admin-secret"

test.describe("S-02: Inactive staff cannot authenticate", () => {
  test("deactivated staff login returns 401 or 403", async () => {
    const { branch, staff } = await seedOrg("s02")

    // Deactivate the staff member
    const deactivateRes = await fetch(`${API_URL}/branches/${branch.id}/staff/${staff.id}/deactivate`, {
      method: "POST",
      headers: { "Authorization": `Bearer ${adminToken}` },
    })
    // Accept 200 or 204 for deactivation success, or 404 if endpoint differs
    expect([200, 204, 404]).toContain(deactivateRes.status)
    if (deactivateRes.status === 404) {
      // Try alternate path
      await fetch(`${API_URL}/staff/${staff.id}/status`, {
        method: "PATCH",
        headers: {
          "Content-Type": "application/json",
          "Authorization": `Bearer ${adminToken}`,
        },
        body: JSON.stringify({ active: false }),
      })
    }

    // Attempt login after deactivation
    const loginRes = await fetch(`${API_URL}/staff/auth`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        branch_code: branch.code,
        staff_code: staff.staffCode,
        pin: staff.pin,
      }),
    })
    expect([401, 403]).toContain(loginRes.status)
  })
})
