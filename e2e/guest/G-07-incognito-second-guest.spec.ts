import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, fetchSnapshot } from "../helpers/api"

test.describe("G-07: Incognito second guest — two distinct participants", () => {
  test("two guests have separate identities and carts", async () => {
    const { table, menu } = await seedOrg("g07")

    // Host creates session
    const hostCreated = await createSession(table.id, "Host")
    const sessionId = hostCreated.session.id
    const hostToken = hostCreated.guest_access_token
    const hostParticipantId = hostCreated.participant.id

    // Second guest joins (incognito = different token)
    const joinRes = await fetch(`${API_URL}/sessions/${sessionId}/join`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ display_name: "GuestB" }),
    })
    expect(joinRes.status).toBe(201)
    const joinData = await joinRes.json()
    const guestBToken = joinData.guest_access_token
    const guestBParticipantId = joinData.participant.id

    // Participants are distinct
    expect(hostParticipantId).not.toBe(guestBParticipantId)

    // Host adds to their own cart
    const addHostRes = await fetch(`${API_URL}/sessions/${sessionId}/cart/items`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${hostToken}`,
      },
      body: JSON.stringify({ menu_item_id: menu.itemId, quantity: 2 }),
    })
    expect(addHostRes.status).toBe(201)

    // GuestB's cart should be empty (separate cart per participant)
    const guestBCart = await fetch(`${API_URL}/sessions/${sessionId}/cart`, {
      headers: { "Authorization": `Bearer ${guestBToken}` },
    })
    expect(guestBCart.status).toBe(200)
    const guestBCartData = await guestBCart.json()
    const guestBItems = guestBCartData.Items ?? guestBCartData.items ?? []
    expect(guestBItems.length).toBe(0)

    // Snapshot shows both participants to host
    const snap = await fetchSnapshot(sessionId, hostToken)
    expect(snap.participants.length).toBe(2)
    expect(snap.participants.some((p) => p.display_name === "Host")).toBe(true)
    expect(snap.participants.some((p) => p.display_name === "GuestB")).toBe(true)
  })
})
