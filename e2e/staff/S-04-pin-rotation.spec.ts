import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, loginStaff } from "../helpers/api"

test.describe("S-04: PIN rotation invalidates old PIN", () => {
  test("old PIN fails after rotation; new PIN succeeds", async () => {
    const { branch, staff } = await seedOrg("s04", { staffRole: "waiter" })

    // Rotate PIN
    const newPin = "999999"
    const rotateRes = await fetch(`${API_URL}/staff/${staff.id}/pin`, {
      method: "PATCH",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${staff.token}`,
      },
      body: JSON.stringify({ current_pin: staff.pin, new_pin: newPin }),
    })
    expect(rotateRes.status).toBe(204)

    // Old PIN should fail
    const oldLoginRes = await fetch(`${API_URL}/staff/auth`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        branch_code: branch.code,
        staff_code: staff.staffCode,
        pin: staff.pin,
      }),
    })
    expect([401, 403]).toContain(oldLoginRes.status)

    // New PIN should succeed
    const newLoginRes = await fetch(`${API_URL}/staff/auth`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        branch_code: branch.code,
        staff_code: staff.staffCode,
        pin: newPin,
      }),
    })
    expect(newLoginRes.status).toBe(200)
    const data = await newLoginRes.json()
    expect(data.token).toBeTruthy()
  })
})
