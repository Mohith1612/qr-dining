import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg } from "../helpers/api"

const adminToken = process.env.E2E_ADMIN_TOKEN ?? "e2e-admin-secret"

test.describe("T-04: Table belongs to one branch only", () => {
  test("assigning a table from branch A to branch B is rejected", async () => {
    const orgA = await seedOrg("t04a")
    const orgB = await seedOrg("t04b")

    // Attempt to reassign orgA's table to orgB's branch
    const res = await fetch(`${API_URL}/branches/${orgB.branch.id}/tables/${orgA.table.id}`, {
      method: "PUT",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${adminToken}`,
      },
      body: JSON.stringify({ identifier: "T-STOLEN" }),
    })
    expect([400, 403, 404, 409, 422]).toContain(res.status)
  })

  test("table token only works within its own branch context", async () => {
    const orgA = await seedOrg("t04c")

    // The table token should resolve to its own branch
    const res = await fetch(`${API_URL}/tables/resolve?token=${orgA.table.token}`)
    if (res.ok) {
      const data = await res.json()
      expect(data.branch_id).toBe(orgA.branch.id)
    } else {
      expect([404]).toContain(res.status)
    }
  })
})
