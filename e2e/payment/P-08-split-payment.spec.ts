import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, placeOrder } from "../helpers/api"
import crypto from "crypto"

test.describe("P-08: Full-bill payment and one outstanding request", () => {
  test("partial amount is rejected and a second initiation returns the existing payment", async () => {
    const { table, menu } = await seedOrg("p08")

    // Host creates session
    const hostCreated = await createSession(table.id, "Host")
    const sessionId = hostCreated.session.id
    const hostToken = hostCreated.guest_access_token

    // Place order
    await placeOrder(sessionId, hostToken, menu.itemId, 2)

    const billRes = await fetch(`${API_URL}/sessions/${sessionId}/bill`, {
      headers: { "Authorization": `Bearer ${hostToken}` },
    })
    expect(billRes.status).toBe(200)
    const bill = await billRes.json() as { total: number }
    expect(bill.total).toBeGreaterThan(0)

    const partialPayment = await fetch(`${API_URL}/sessions/${sessionId}/payments`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${hostToken}`,
      },
      body: JSON.stringify({
        amount: Math.round((bill.total - 0.01) * 100) / 100,
        method: "cash",
        idempotency_key: crypto.randomUUID(),
      }),
    })
    expect(partialPayment.status).toBe(422)
    const partialError = await partialPayment.json()
    expect(partialError.code).toBe("PAYMENT_AMOUNT_INVALID")

    const firstPayment = await fetch(`${API_URL}/sessions/${sessionId}/payments`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${hostToken}`,
      },
      body: JSON.stringify({
        amount: bill.total,
        method: "cash",
        idempotency_key: crypto.randomUUID(),
      }),
    })
    expect(firstPayment.status).toBe(201)
    const first = await firstPayment.json() as { id: number; status: string }
    expect(first.id).toBeGreaterThan(0)
    expect(first.status).toBe("requires_staff_confirmation")

    const secondPayment = await fetch(`${API_URL}/sessions/${sessionId}/payments`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${hostToken}`,
      },
      body: JSON.stringify({
        amount: bill.total,
        method: "cash",
        idempotency_key: crypto.randomUUID(),
      }),
    })
    expect(secondPayment.status).toBe(200)
    const second = await secondPayment.json() as { id: number; status: string }
    expect(second.id).toBe(first.id)
    expect(second.status).toBe("requires_staff_confirmation")
  })
})
