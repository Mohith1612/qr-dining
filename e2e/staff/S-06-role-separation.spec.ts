import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, loginStaff } from "../helpers/api"

const adminToken = process.env.E2E_ADMIN_TOKEN ?? "e2e-admin-secret"

test.describe("S-06: Role separation — waiter cannot access admin endpoints", () => {
  test("waiter token rejected on branch admin operations", async () => {
    const { branch, staff } = await seedOrg("s06")

    const waiterCtx = await loginStaff(branch.code, staff.staffCode, staff.pin)
    expect(waiterCtx.role).toBe("waiter")

    // Waiter tries to create another staff member — should be forbidden
    const res = await fetch(`${API_URL}/branches/${branch.id}/staff`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${waiterCtx.token}`,
      },
      body: JSON.stringify({
        name: "Unauthorized Staff",
        role: "waiter",
        staff_code: `unauth-${Date.now()}`,
        pin: "111111",
      }),
    })
    expect([401, 403]).toContain(res.status)
  })

  test("waiter token rejected on menu management", async () => {
    const { branch, staff, menu } = await seedOrg("s06b")

    const waiterCtx = await loginStaff(branch.code, staff.staffCode, staff.pin)

    // Waiter tries to update menu item price
    const res = await fetch(`${API_URL}/branches/${branch.id}/menu/items/${menu.itemId}`, {
      method: "PATCH",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${waiterCtx.token}`,
      },
      body: JSON.stringify({ price: 1 }),
    })
    expect([401, 403]).toContain(res.status)
  })
})
