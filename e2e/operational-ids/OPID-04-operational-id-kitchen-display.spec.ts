import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, placeOrder, loginStaff } from "../helpers/api"

test.describe("OPID-04: Operational ID visible to kitchen staff", () => {
  // VACUOUS(sig-3): accepts a missing endpoint and asserts field-present-or-absent; passes with no kitchen ID.
  test.fixme("kitchen orders list includes operational_id for display", async () => {
    const { table, menu, branch, staff } = await seedOrg("opid04")
    const created = await createSession(table.id, "KitchenGuest")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    await placeOrder(sessionId, guestToken, menu.itemId, 1)

    const kitchenStaffCode = `kitchen-${Date.now()}`
    const adminToken = process.env.E2E_ADMIN_TOKEN ?? "e2e-admin-secret"
    await fetch(`${API_URL}/branches/${branch.id}/staff`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${adminToken}`,
      },
      body: JSON.stringify({
        name: "Kitchen Staff",
        role: "kitchen",
        staff_code: kitchenStaffCode,
        pin: "444444",
      }),
    })

    const kitchenCtx = await loginStaff(branch.code, kitchenStaffCode, "444444")

    const ordersRes = await fetch(`${API_URL}/staff/orders?branch_id=${branch.id}`, {
      headers: { "Authorization": `Bearer ${kitchenCtx.token}` },
    })

    if (ordersRes.ok) {
      const data = await ordersRes.json()
      const orders = data.orders ?? data.items ?? data
      if (Array.isArray(orders) && orders.length > 0) {
        const firstOrder = orders[0]
        const opId = firstOrder.operational_id ?? firstOrder.order_operational_id
        expect(opId !== undefined || opId === undefined).toBe(true)
      }
    } else {
      expect([200, 404]).toContain(ordersRes.status)
    }
  })
})
