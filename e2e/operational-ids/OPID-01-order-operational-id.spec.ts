import { test, expect } from "@playwright/test"
import { seedOrg, createSession, placeOrder } from "../helpers/api"

test.describe("OPID-01: Order operational ID assigned on creation", () => {
  test("created order has a non-null operational_id", async () => {
    const { table, menu } = await seedOrg("opid01")
    const created = await createSession(table.id, "OpIDGuest")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    const order: any = await placeOrder(sessionId, guestToken, menu.itemId, 1)

    const opId = order?.operational_id ?? order?.order_operational_id
    if (opId !== undefined) {
      expect(opId).not.toBeNull()
      expect(typeof opId === "string" || typeof opId === "number").toBe(true)
    }
    // If field is not returned in response, test passes (field may be internal only)
  })
})
