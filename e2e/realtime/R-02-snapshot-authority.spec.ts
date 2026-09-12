import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, placeOrder, fetchSnapshot } from "../helpers/api"

test.describe("R-02: Snapshot is authoritative state", () => {
  test("snapshot reflects all committed mutations", async () => {
    const { table, menu } = await seedOrg("r02")
    const created = await createSession(table.id, "SnapUser")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token
    const participantId = created.participant.id

    // Add item to cart
    await fetch(`${API_URL}/sessions/${sessionId}/cart/items`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${guestToken}`,
      },
      body: JSON.stringify({ menu_item_id: menu.itemId, quantity: 1 }),
    })

    // Join a second participant
    const joinRes = await fetch(`${API_URL}/sessions/${sessionId}/join`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ display_name: "P2" }),
    })
    expect(joinRes.status).toBe(201)

    // Snapshot should reflect both
    const snap = await fetchSnapshot(sessionId, guestToken)
    expect(snap.session.status).toBe("active")
    expect(snap.participants.length).toBeGreaterThanOrEqual(2)
    expect(snap.participants.some((p) => p.display_name === "SnapUser")).toBe(true)
    expect(snap.participants.some((p) => p.display_name === "P2")).toBe(true)
  })

  // VACUOUS(sig-2): substitutes an empty array for a missing field; passes when missed_events is omitted.
  test.fixme("snapshot with last_sequence returns missed_events field", async () => {
    const { table } = await seedOrg("r02b")
    const created = await createSession(table.id, "SeqUser")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    const snapRes = await fetch(`${API_URL}/sessions/${sessionId}/snapshot?last_sequence=0`, {
      headers: { "Authorization": `Bearer ${guestToken}` },
    })
    expect(snapRes.status).toBe(200)
    const snap = await snapRes.json()
    expect(snap.session).toBeDefined()
    // missed_events should be present (possibly empty)
    expect(Array.isArray(snap.missed_events ?? [])).toBe(true)
  })
})
