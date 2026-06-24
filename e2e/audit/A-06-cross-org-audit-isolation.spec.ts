import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, loginStaff } from "../helpers/api"

test.describe("A-06: Cross-org audit isolation — org owner cannot read other org audit", () => {
  test("owner of org A cannot read audit logs of org B sessions", async () => {
    const orgA = await seedOrg("a06a")
    const orgB = await seedOrg("a06b")

    const sessionB = await createSession(orgB.table.id, "AuditGuestB")

    // Seed an owner for org A
    const adminToken = process.env.E2E_ADMIN_TOKEN ?? "e2e-admin-secret"
    const ownerCode = `owner-a06-${Date.now()}`
    await fetch(`${API_URL}/branches/${orgA.branch.id}/staff`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${adminToken}`,
      },
      body: JSON.stringify({
        name: "OrgA Owner",
        role: "owner",
        staff_code: ownerCode,
        pin: "555555",
      }),
    })

    const ownerCtx = await loginStaff(orgA.branch.code, ownerCode, "555555")

    // Owner A tries to read Org B's session audit
    const res = await fetch(
      `${API_URL}/platform/audit?resource_type=session&resource_id=${sessionB.session.id}`,
      { headers: { "Authorization": `Bearer ${ownerCtx.token}` } }
    )
    expect([401, 403, 404]).toContain(res.status)
  })
})
