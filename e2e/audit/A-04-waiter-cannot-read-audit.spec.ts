import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, loginStaff, createSession } from "../helpers/api"

test.describe("A-04: Waiter-role staff cannot read audit logs", () => {
  test("GET /platform/audit with waiter token returns 401 or 403", async () => {
    const { branch, staff } = await seedOrg("a04")
    const waiterCtx = await loginStaff(branch.code, staff.staffCode, staff.pin)

    const res = await fetch(`${API_URL}/platform/audit?resource_type=session&resource_id=any`, {
      headers: { "Authorization": `Bearer ${waiterCtx.token}` },
    })
    expect([401, 403]).toContain(res.status)
  })
})
