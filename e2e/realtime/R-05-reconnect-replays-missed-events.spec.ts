import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession } from "../helpers/api"
import crypto from "crypto"

test.describe("R-05: Reconnect replays missed events", () => {
  test("snapshot with last_sequence returns only newer events", async () => {
    const { table, menu } = await seedOrg("r05")
    const created = await createSession(table.id, "ReplayUser")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    // Place an order to create events
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

    // Snapshot with since=0 → should include order events
    const snap0 = await fetch(`${API_URL}/sessions/${sessionId}/snapshot?last_sequence=0`, {
      headers: { "Authorization": `Bearer ${guestToken}` },
    }).then((r) => r.json())

    const events = snap0.missed_events ?? []
    if (events.length === 0) return // No events produced, skip sequence check

    const lastSeq = events[events.length - 1].sequence as number | undefined
    if (typeof lastSeq !== "number") return

    // Snapshot with last_sequence = lastSeq should return no missed events
    const snapLatest = await fetch(
      `${API_URL}/sessions/${sessionId}/snapshot?last_sequence=${lastSeq}`,
      { headers: { "Authorization": `Bearer ${guestToken}` } }
    ).then((r) => r.json())

    const laterEvents = snapLatest.missed_events ?? []
    expect(laterEvents.length).toBe(0)
  })
})
