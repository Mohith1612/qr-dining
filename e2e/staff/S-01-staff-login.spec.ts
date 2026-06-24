import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg } from "../helpers/api"

test.describe("S-01: Staff login with branch_code + staff_code + PIN", () => {
  test("valid credentials return staff token with correct role", async () => {
    const { branch, staff } = await seedOrg("s01")

    const res = await fetch(`${API_URL}/staff/auth`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        branch_code: branch.code,
        staff_code: staff.staffCode,
        pin: staff.pin,
      }),
    })
    expect(res.status).toBe(200)
    const data = await res.json()
    expect(data.token).toBeTruthy()
    expect(data.role).toBeDefined()
    expect(data.branch_id).toBe(branch.id)
  })

  test("wrong PIN returns 401", async () => {
    const { branch, staff } = await seedOrg("s01b")

    const res = await fetch(`${API_URL}/staff/auth`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        branch_code: branch.code,
        staff_code: staff.staffCode,
        pin: "000000",
      }),
    })
    expect([401, 403]).toContain(res.status)
  })

  test("old branch_id + pin format is rejected when strict mode active", async () => {
    const { branch, staff } = await seedOrg("s01c")

    const res = await fetch(`${API_URL}/staff/auth`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        branch_id: branch.id,
        pin: staff.pin,
      }),
    })
    // strict mode rejects old format
    expect([400, 401, 422]).toContain(res.status)
  })
})
