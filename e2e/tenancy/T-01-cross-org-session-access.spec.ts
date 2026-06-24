import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, loginStaff } from "../helpers/api"

test.describe("T-01: Cross-org session access denied", () => {
  test("staff from org A cannot read org B session snapshot", async () => {
    const orgA = await seedOrg("t01a")
    const orgB = await seedOrg("t01b")

    const sessionB = await createSession(orgB.table.id, "GuestOrgB")
    const staffA = await loginStaff(orgA.branch.code, orgA.staff.staffCode, orgA.staff.pin)

    const res = await fetch(`${API_URL}/sessions/${sessionB.session.id}/snapshot`, {
      headers: { "Authorization": `Bearer ${staffA.token}` },
    })
    expect([401, 403, 404]).toContain(res.status)
  })
})
