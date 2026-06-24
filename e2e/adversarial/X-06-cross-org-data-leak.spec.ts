import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, loginStaff } from "../helpers/api"

test.describe("X-06: Cross-org data leak prevention", () => {
  test("staff from org A cannot read org B resources", async () => {
    const orgA = await seedOrg("x06a")
    const orgB = await seedOrg("x06b")

    const staffACtx = await loginStaff(orgA.branch.code, orgA.staff.staffCode, orgA.staff.pin)

    // Org A staff tries to read Org B's active sessions
    const res = await fetch(`${API_URL}/branches/${orgB.branch.id}/sessions/active`, {
      headers: { "Authorization": `Bearer ${staffACtx.token}` },
    })
    expect([403, 404]).toContain(res.status)

    // Org A staff tries to read Org B's menu
    const menuRes = await fetch(`${API_URL}/branches/${orgB.branch.id}/menu/full`, {
      headers: { "Authorization": `Bearer ${staffACtx.token}` },
    })
    expect([403, 404]).toContain(menuRes.status)
  })
})
