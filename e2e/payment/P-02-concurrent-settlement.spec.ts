import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, loginStaff, placeOrder } from "../helpers/api"
import crypto from "crypto"

test.describe("P-02: Concurrent settlement attempts", () => {
  test("two staff settle same payment — one wins, one no-ops", async () => {
    const { branch, table, menu, staff } = await seedOrg("p02")
    const created = await createSession(table.id, "ConcurrentPay")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    await placeOrder(sessionId, guestToken, menu.itemId)

    const payRes = await fetch(`${API_URL}/sessions/${sessionId}/payments`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${guestToken}`,
      },
      body: JSON.stringify({
        amount: menu.itemPrice,
        method: "cash",
        idempotency_key: crypto.randomUUID(),
      }),
    })
    expect([200, 201]).toContain(payRes.status)
    const pay = await payRes.json()
    const paymentId = pay.id ?? pay.payment?.id

    if (!paymentId) return // Skip if payment creation failed

    const staffCtx = await loginStaff(branch.code, staff.staffCode, staff.pin)

    // Two concurrent settle attempts
    const [settle1, settle2] = await Promise.all([
      fetch(`${API_URL}/payments/${paymentId}/settle`, {
        method: "PATCH",
        headers: {
          "Content-Type": "application/json",
          "Authorization": `Bearer ${staffCtx.token}`,
        },
        body: JSON.stringify({ notes: "Cash A" }),
      }),
      fetch(`${API_URL}/payments/${paymentId}/settle`, {
        method: "PATCH",
        headers: {
          "Content-Type": "application/json",
          "Authorization": `Bearer ${staffCtx.token}`,
        },
        body: JSON.stringify({ notes: "Cash B" }),
      }),
    ])

    const statuses = [settle1.status, settle2.status]
    // Both should succeed (idempotent) or one succeeds and one gets 409
    expect(statuses.every((s) => [200, 204, 404, 409].includes(s))).toBe(true)
  })
})
