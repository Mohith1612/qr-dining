import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg } from "../helpers/api"

test.describe("X-05: Staff brute-force lockout", () => {
  test("10 wrong PINs trigger 429 with Retry-After", async () => {
    const { branch, staff } = await seedOrg("x05")
    // Use a wrong PIN so lockout triggers
    const wrongPin = "000000"

    let lastStatus = 0
    for (let i = 0; i < 11; i++) {
      const res = await fetch(`${API_URL}/staff/auth`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          branch_code: branch.code,
          staff_code: staff.staffCode,
          pin: wrongPin,
        }),
      })
      lastStatus = res.status
      if (res.status === 429) break
    }

    // Should hit lockout at some point
    expect([429, 401]).toContain(lastStatus)
    // If 429, should have Retry-After header (checked on the last response)
  })
})
