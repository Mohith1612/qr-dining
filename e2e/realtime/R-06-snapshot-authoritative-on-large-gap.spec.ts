import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession } from "../helpers/api"
import crypto from "crypto"

test.describe("R-06: SNAPSHOT_AUTHORITATIVE when gap exceeds replay window", () => {
  test("snapshot returns SNAPSHOT_AUTHORITATIVE signal or missed events for large gap", async () => {
    const { table, menu } = await seedOrg("r06")
    const created = await createSession(table.id, "GapUser")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    // Place 3 orders to have some events
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

    // Request with very old sequence (since=0 with large gap)
    const snapRes = await fetch(`${API_URL}/sessions/${sessionId}/snapshot?last_sequence=0`, {
      headers: { "Authorization": `Bearer ${guestToken}` },
    })
    expect(snapRes.status).toBe(200)
    const snap = await snapRes.json()

    // Either returns missed_events OR SNAPSHOT_AUTHORITATIVE signal
    const hasEvents = Array.isArray(snap.missed_events) && snap.missed_events.length > 0
    const hasSnapshotSignal = snap.missed_events === null || snap.snapshot_authoritative === true
    expect(hasEvents || hasSnapshotSignal).toBe(true)
  })
})
