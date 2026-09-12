import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, placeOrder } from "../helpers/api"

test.describe("OPID-04: Operational ID visible to kitchen staff", () => {
  // VACUOUS(sig-3): accepts a missing endpoint and asserts field-present-or-absent; passes with no kitchen ID.
  test.fixme("kitchen orders list includes operational_id for display", async () => {
    const { table, menu, branch, staff } = await seedOrg("opid04", { staffRole: "kitchen" })
    const created = await createSession(table.id, "KitchenGuest")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    await placeOrder(sessionId, guestToken, menu.itemId, 1)

    const ordersRes = await fetch(`${API_URL}/staff/orders?branch_id=${branch.id}`, {
      headers: { "Authorization": `Bearer ${staff.token}` },
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
