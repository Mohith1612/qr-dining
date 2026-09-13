import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg } from "../helpers/api"

test.describe("X-05: Staff brute-force lockout", () => {
  test("10 wrong PINs trigger 423 with Retry-After", async () => {
    const { branch, staff } = await seedOrg("x05")
    const wrongPin = "000000"

    const responses: Response[] = []
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
      responses.push(res)
    }

    expect(responses.slice(0, 10).map((response) => response.status)).toEqual(
      Array(10).fill(401)
    )
    const lockedResponse = responses[10]
    expect(lockedResponse.status).toBe(423)
    const retryAfter = lockedResponse.headers.get("Retry-After")
    expect(retryAfter).toMatch(/^\d+$/)
    expect(Number(retryAfter)).toBeGreaterThan(0)
  })
})
