import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg } from "../helpers/api"

test.describe("A-04: Waiter-role staff cannot read audit logs", () => {
  test("GET branch audit with waiter token returns 403", async () => {
    const { branch, staff } = await seedOrg("a04", { staffRole: "waiter" })

    const res = await fetch(`${API_URL}/branches/${branch.id}/audit`, {
      headers: { "Authorization": `Bearer ${staff.token}` },
    })
    expect(res.status).toBe(403)
  })
})
