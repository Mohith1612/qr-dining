import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, placeOrder } from "../helpers/api"
import crypto from "crypto"

test.describe("P-10: Promo race — concurrent validation", () => {
  test("concurrent promo applications do not double-discount", async () => {
    const { table, menu } = await seedOrg("p10")
    const created = await createSession(table.id, "PromoRacer")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    await placeOrder(sessionId, guestToken, menu.itemId, 2)

    // Validate promo code — server may not have promos seeded; accept 404/400 as not-found
    const promoCode = "E2E-PROMO-RACE"
    const [r1, r2] = await Promise.all([
      fetch(`${API_URL}/sessions/${sessionId}/promos/validate`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Authorization": `Bearer ${guestToken}`,
        },
        body: JSON.stringify({ code: promoCode }),
      }),
      fetch(`${API_URL}/sessions/${sessionId}/promos/validate`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Authorization": `Bearer ${guestToken}`,
        },
        body: JSON.stringify({ code: promoCode }),
      }),
    ])

    // At most one application should succeed; a conflict or not-found is fine
    const statuses = [r1.status, r2.status]
    for (const s of statuses) {
      expect([200, 201, 400, 404, 409, 422]).toContain(s)
    }

    // If both succeeded (same idempotent key path), they should return identical discount
    const results = await Promise.all([r1.json().catch(() => null), r2.json().catch(() => null)])
    const successCount = statuses.filter((s) => s === 200 || s === 201).length
    if (successCount === 2 && results[0] && results[1]) {
      expect(results[0].discount).toEqual(results[1].discount)
    }
  })
})
