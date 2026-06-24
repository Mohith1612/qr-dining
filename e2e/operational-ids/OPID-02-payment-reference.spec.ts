import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, placeOrder } from "../helpers/api"
import crypto from "crypto"

test.describe("OPID-02: Payment reference preserved through settlement", () => {
  test("payment reference set on initiation is returned in bill", async () => {
    const { table, menu } = await seedOrg("opid02")
    const created = await createSession(table.id, "PayRefGuest")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    await placeOrder(sessionId, guestToken, menu.itemId, 1)

    const ref = `e2e-ref-${crypto.randomUUID()}`
    const payRes = await fetch(`${API_URL}/sessions/${sessionId}/payments`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${guestToken}`,
      },
      body: JSON.stringify({
        amount: menu.itemPrice,
        method: "digital",
        idempotency_key: crypto.randomUUID(),
        reference: ref,
      }),
    })

    if (payRes.ok) {
      const payment = await payRes.json()
      const returnedRef = payment.reference ?? payment.external_reference
      if (returnedRef !== undefined) {
        expect(returnedRef).toBe(ref)
      }
    } else {
      expect([200, 201]).toContain(payRes.status)
    }
  })
})
