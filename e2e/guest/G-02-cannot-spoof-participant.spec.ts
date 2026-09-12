import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession } from "../helpers/api"

// The invariant: order attribution comes from the guest credential, never from
// the request body. A participant_id in the body is ignored.
//
// This spec used to make a NON-host guest attempt the spoof. Since host-only
// ordering landed that request is refused with 403 NOT_SESSION_HOST before
// attribution is ever decided, so the spoofing assertion sat in an unreachable
// branch. It also read `order.Order.participant_id` — a field that has never
// existed on sqlc.Order (the column is placed_by_participant_id), so it was
// `undefined` regardless. The spoofer is now the host, who is the only actor
// who can place an order, and the assertion reads the real field.

test.describe("G-02: Guest cannot spoof another participant", () => {
  test("order attributed to token identity, not body participant_id", async () => {
    const { table, menu } = await seedOrg("g02")

    // Two guests in the same session: the host (who may order) and a joiner.
    const host = await createSession(table.id, "Host")
    const sessionId = host.session.id
    const hostParticipantId = host.participant.id
    const hostToken = host.guest_access_token

    const guestRes = await fetch(`${API_URL}/sessions/${sessionId}/join`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ display_name: "Guest" }),
    })
    expect(guestRes.status).toBe(201)
    const otherParticipantId = (await guestRes.json()).participant.id
    expect(otherParticipantId).not.toBe(hostParticipantId)

    // The host orders, but claims the order belongs to the other participant.
    const orderRes = await fetch(`${API_URL}/sessions/${sessionId}/orders`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${hostToken}`,
      },
      body: JSON.stringify({
        idempotency_key: crypto.randomUUID(),
        items: [{ menu_item_id: menu.itemId, quantity: 1 }],
        // Legacy attempt to spoof a different participant.
        participant_id: otherParticipantId,
      }),
    })
    expect(orderRes.status).toBe(201)

    const result = await orderRes.json() as {
      order: { id: string; placed_by_participant_id: number }
    }
    // Attribution follows the bearer token, not the body.
    expect(result.order.placed_by_participant_id).toBe(hostParticipantId)
    expect(result.order.placed_by_participant_id).not.toBe(otherParticipantId)
  })

  test("a non-host cannot place an order at all, spoofed body or not", async () => {
    const { table, menu } = await seedOrg("g02b")
    const host = await createSession(table.id, "Host")
    const sessionId = host.session.id

    const guestRes = await fetch(`${API_URL}/sessions/${sessionId}/join`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ display_name: "Guest" }),
    })
    const guestToken = (await guestRes.json()).guest_access_token

    const orderRes = await fetch(`${API_URL}/sessions/${sessionId}/orders`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${guestToken}`,
      },
      body: JSON.stringify({
        idempotency_key: crypto.randomUUID(),
        items: [{ menu_item_id: menu.itemId, quantity: 1 }],
        participant_id: host.participant.id,
      }),
    })
    expect(orderRes.status).toBe(403)
    expect((await orderRes.json()).code).toBe("NOT_SESSION_HOST")
  })
})
