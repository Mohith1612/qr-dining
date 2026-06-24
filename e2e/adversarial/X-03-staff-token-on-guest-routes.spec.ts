import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, loginStaff } from "../helpers/api"

test.describe("X-03: Staff token rejected on guest routes", () => {
  test("staff token on guest session endpoint returns 401/403", async () => {
    const { branch, table, staff } = await seedOrg("x03")
    const created = await createSession(table.id, "GuestX")
    const sessionId = created.session.id

    const staffCtx = await loginStaff(branch.code, staff.staffCode, staff.pin)

    // Staff token used on guest cart endpoint
    const cartRes = await fetch(`${API_URL}/sessions/${sessionId}/cart`, {
      headers: { "Authorization": `Bearer ${staffCtx.token}` },
    })
    // Backend should reject staff tokens on guest-scoped endpoints
    expect([401, 403]).toContain(cartRes.status)
  })
})
