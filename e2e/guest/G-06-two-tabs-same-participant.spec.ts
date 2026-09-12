import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession } from "../helpers/api"

test.describe("G-06: Two tabs same participant — cart sync", () => {
  // VACUOUS(sig-7): claims two-tab behavior but performs only HTTP requests; passes when browser tab sync is broken.
  test.fixme("cart update in one tab reflects in snapshot for other tab", async ({ browser }) => {
    const { table, menu } = await seedOrg("g06")
    const created = await createSession(table.id, "TabUser")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    // Simulate tab A: add item to cart
    const addRes = await fetch(`${API_URL}/sessions/${sessionId}/cart/items`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${guestToken}`,
      },
      body: JSON.stringify({ menu_item_id: menu.itemId, quantity: 1 }),
    })
    expect(addRes.status).toBe(201)

    // Simulate tab B: read cart — should see the item
    const cartRes = await fetch(`${API_URL}/sessions/${sessionId}/cart`, {
      headers: { "Authorization": `Bearer ${guestToken}` },
    })
    expect(cartRes.status).toBe(200)
    // snake_case per the CartResponse tags (F-26). Read `items` directly: the
    // old `cart.Items ?? cart.items ?? []` chain would fall through to an empty
    // array if both names were wrong, turning the length check into a silent
    // pass rather than a failure.
    const cart = await cartRes.json() as {
      items: Array<{ menu_item_id: number }>
    }
    const items = cart.items
    expect(Array.isArray(items)).toBe(true)
    expect(items.length).toBeGreaterThanOrEqual(1)
    expect(items.some((i) => i.menu_item_id === menu.itemId)).toBe(true)
  })
})
