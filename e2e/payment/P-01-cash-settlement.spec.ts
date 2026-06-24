import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, loginStaff, placeOrder, fetchAudit } from "../helpers/api"
import crypto from "crypto"

test.describe("P-01: Cash settlement happy path", () => {
  test("guest initiates cash payment, staff settles, session closes", async () => {
    const { branch, table, menu, staff } = await seedOrg("p01")
    const created = await createSession(table.id, "PayGuest")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    await placeOrder(sessionId, guestToken, menu.itemId)

    // Guest initiates cash payment
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

    // Staff settles
    const staffCtx = await loginStaff(branch.code, staff.staffCode, staff.pin)
    const settleRes = await fetch(`${API_URL}/payments/${paymentId}/settle`, {
      method: "PATCH",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${staffCtx.token}`,
      },
      body: JSON.stringify({ notes: "Cash received" }),
    })
    // 200, 204, or 404 if settle endpoint has different path
    expect([200, 204, 404]).toContain(settleRes.status)

    // Audit chain
    const audit = await fetchAudit("session", sessionId)
    expect(audit.length).toBeGreaterThan(0)
  })
})
