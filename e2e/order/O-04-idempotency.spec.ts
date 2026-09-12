import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession } from "../helpers/api"
import crypto from "crypto"

test.describe("O-04: Order idempotency — same key produces same order", () => {
  // VACUOUS(sig-4): compares response fields the order API does not return; passes as undefined equals undefined.
  test.fixme("duplicate order with same idempotency key does not create a second order", async () => {
    const { table, menu } = await seedOrg("o04")
    const created = await createSession(table.id, "IdempotGuest")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    const key = crypto.randomUUID()
    const body = JSON.stringify({
      idempotency_key: key,
      items: [{ menu_item_id: menu.itemId, quantity: 1 }],
    })

    const place = () =>
      fetch(`${API_URL}/sessions/${sessionId}/orders`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Authorization": `Bearer ${guestToken}`,
        },
        body,
      })

    const r1 = await place()
    const r2 = await place()

    expect([200, 201]).toContain(r1.status)
    // Idempotent repeat: same 2xx or 409 CONFLICT
    expect([200, 201, 409]).toContain(r2.status)

    const o1 = await r1.json()
    if (r2.status !== 409) {
      const o2 = await r2.json()
      expect(o1.id ?? o1.order_id).toEqual(o2.id ?? o2.order_id)
    }
  })

  test("same key with different body returns 409 IDEMPOTENCY_CONFLICT", async () => {
    const { table, menu } = await seedOrg("o04b")
    const created = await createSession(table.id, "IdempotConflict")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    const key = crypto.randomUUID()

    await fetch(`${API_URL}/sessions/${sessionId}/orders`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${guestToken}`,
      },
      body: JSON.stringify({
        idempotency_key: key,
        items: [{ menu_item_id: menu.itemId, quantity: 1 }],
      }),
    })

    const conflictRes = await fetch(`${API_URL}/sessions/${sessionId}/orders`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${guestToken}`,
      },
      body: JSON.stringify({
        idempotency_key: key,
        items: [{ menu_item_id: menu.itemId, quantity: 99 }],
      }),
    })
    expect([409]).toContain(conflictRes.status)
  })
})
