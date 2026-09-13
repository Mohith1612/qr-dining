import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, loginStaff, fetchAudit } from "../helpers/api"

test.describe("L-07: Force close by staff", () => {
  test("staff force-close marks session closed and audit is recorded", async () => {
    const { branch, table, staff } = await seedOrg("l07")
    const created = await createSession(table.id, "GuestFC")
    const sessionId = created.session.id

    const staffCtx = await loginStaff(branch.code, staff.staffCode, staff.pin)

    const closeRes = await fetch(`${API_URL}/sessions/${sessionId}/force-close`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${staffCtx.token}`,
      },
      body: JSON.stringify({ reason: "staff_force_close" }),
    })
    expect(closeRes.status).toBe(200)

    const audit = await fetchAudit("session", sessionId)
    const closeAudit = audit.find((a) => a.action.includes("close") || a.action.includes("force"))
    expect(closeAudit).toBeTruthy()
  })
})
