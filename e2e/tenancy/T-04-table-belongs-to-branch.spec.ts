import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg } from "../helpers/api"

test.describe("T-04: Table belongs to one branch only", () => {
  // VACUOUS(sig-5): mutates only the identifier and never attempts branch reassignment; passes without ownership enforcement.
  test.fixme("assigning a table from branch A to branch B is rejected", async () => {
    const orgA = await seedOrg("t04a")
    const orgB = await seedOrg("t04b")

    // Attempt to update orgA's table with orgB's staff credential.
    const res = await fetch(`${API_URL}/tables/${orgA.table.id}`, {
      method: "PATCH",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${orgB.owner.token}`,
      },
      body: JSON.stringify({ identifier: "T-STOLEN" }),
    })
    expect(res.status).toBe(403)
  })

  // VACUOUS(sig-2): accepts 404 and asserts branch ownership only on success; passes when QR resolution is absent.
  test.fixme("table token only works within its own branch context", async () => {
    const orgA = await seedOrg("t04c")

    // The table token should resolve to its own branch
    const res = await fetch(`${API_URL}/tables/by-qr/${orgA.table.token}`)
    if (res.ok) {
      const data = await res.json()
      expect(data.branch_id).toBe(orgA.branch.id)
    } else {
      expect([404]).toContain(res.status)
    }
  })
})
