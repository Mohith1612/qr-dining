import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, placeOrder } from "../helpers/api"
import crypto from "crypto"

test.describe("R-04: Event sequences are monotonically increasing", () => {
  // VACUOUS(sig-2): asserts only when at least two numeric events appear; passes when replay returns no events.
  test.fixme("missed_events from snapshot are in ascending sequence order", async () => {
    const { table, menu } = await seedOrg("r04")
    const created = await createSession(table.id, "SeqUser")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    // Place 3 orders to generate events
    for (let i = 0; i < 3; i++) {
      await fetch(`${API_URL}/sessions/${sessionId}/orders`, {
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
    }

    // Get snapshot with since=0 to get all events
    const snapRes = await fetch(`${API_URL}/sessions/${sessionId}/snapshot?last_sequence=0`, {
      headers: { "Authorization": `Bearer ${guestToken}` },
    })
    expect(snapRes.status).toBe(200)
    const snap = await snapRes.json()

    const events = snap.missed_events ?? []
    if (events.length >= 2) {
      // Verify sequences are ascending
      for (let i = 1; i < events.length; i++) {
        const prev = events[i - 1].sequence
        const curr = events[i].sequence
        if (typeof prev === "number" && typeof curr === "number") {
          expect(curr).toBeGreaterThan(prev)
        }
      }
    }
  })
})
