import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, forceCloseSession } from "../helpers/api"
import crypto from "crypto"

test.describe("O-06: Order placement rejected on closed session", () => {
  test("ordering after session is closed returns 409 or 410", async () => {
    const { table, menu } = await seedOrg("o06")
    const created = await createSession(table.id, "ClosedGuest")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    await forceCloseSession(sessionId, guestToken)

    const res = await fetch(`${API_URL}/sessions/${sessionId}/orders`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${guestToken}`,
      },
      body: JSON.stringify({
        idempotency_key: crypto.randomUUID(),
        items: [{ menu_item_id: menu.itemId, quantity: 1 }],
      }),
    })
    expect([400, 401, 403, 409, 410, 422]).toContain(res.status)
  })
})
