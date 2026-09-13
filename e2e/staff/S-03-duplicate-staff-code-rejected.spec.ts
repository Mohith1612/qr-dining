import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg } from "../helpers/api"

test.describe("S-03: Duplicate staff_code within branch rejected", () => {
  test("creating two staff with the same staff_code returns 409", async () => {
    const { branch, owner } = await seedOrg("s03")
    const sharedCode = `staff-dupe-${Date.now()}`

    const first = await fetch(`${API_URL}/branches/${branch.id}/staff`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${owner.token}`,
      },
      body: JSON.stringify({
        name: "Staff One",
        role: "waiter",
        staff_code: sharedCode,
        pin: "111111",
      }),
    })
    expect(first.status).toBe(201)

    const second = await fetch(`${API_URL}/branches/${branch.id}/staff`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${owner.token}`,
      },
      body: JSON.stringify({
        name: "Staff Two",
        role: "waiter",
        staff_code: sharedCode,
        pin: "222222",
      }),
    })
    expect([409, 422]).toContain(second.status)
  })
})
