import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, loginStaff, createSession } from "../helpers/api"

test.describe("S-07: Staff cannot mutate data from another branch", () => {
  test("staff token from branch A rejected on branch B session operations", async () => {
    const orgA = await seedOrg("s07a")
    const orgB = await seedOrg("s07b")

    const staffA = await loginStaff(orgA.branch.code, orgA.staff.staffCode, orgA.staff.pin)

    // Create a session under branch B
    const sessionB = await createSession(orgB.table.id, "GuestB")
    const sessionBId = sessionB.session.id

    // Staff A tries to force-close Branch B's session
    const res = await fetch(`${API_URL}/staff/sessions/${sessionBId}/close`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${staffA.token}`,
      },
      body: JSON.stringify({ reason: "test" }),
    })
    expect([401, 403, 404]).toContain(res.status)
  })
})
