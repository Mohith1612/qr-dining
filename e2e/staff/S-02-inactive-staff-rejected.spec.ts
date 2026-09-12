import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg } from "../helpers/api"

test.describe("S-02: Inactive staff cannot authenticate", () => {
  test("deactivated staff login returns 401 or 403", async () => {
    const { branch, owner, staff } = await seedOrg("s02", { staffRole: "waiter" })

    // Deactivate the staff member
    const deactivateRes = await fetch(`${API_URL}/staff/${staff.id}/deactivate`, {
      method: "PATCH",
      headers: { "Authorization": `Bearer ${owner.token}` },
    })
    expect(deactivateRes.status).toBe(204)

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
