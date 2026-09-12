import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession } from "../helpers/api"

test.describe("A-06: Cross-org audit isolation — org owner cannot read other org audit", () => {
  test("owner of org A cannot read audit logs of org B sessions", async () => {
    const orgA = await seedOrg("a06a")
    const orgB = await seedOrg("a06b")

    await createSession(orgB.table.id, "AuditGuestB")

    // Owner A tries to read Org B's session audit
    const res = await fetch(`${API_URL}/branches/${orgB.branch.id}/audit`, {
      headers: { "Authorization": `Bearer ${orgA.owner.token}` },
    })
    expect(res.status).toBe(403)
  })
})
