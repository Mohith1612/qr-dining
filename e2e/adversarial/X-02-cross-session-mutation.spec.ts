import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession } from "../helpers/api"

test.describe("X-02: Cross-session mutation attempt", () => {
  test("guest token from session A rejected by session B endpoints", async () => {
    const { table: tableA } = await seedOrg("x02a")
    const { table: tableB } = await seedOrg("x02b")

    const sessionA = await createSession(tableA.id, "GuestA")
    const sessionB = await createSession(tableB.id, "GuestB")

    const tokenA = sessionA.guest_access_token
    const sessionBId = sessionB.session.id

    // Token from session A used against session B cart
    const res = await fetch(`${API_URL}/sessions/${sessionBId}/cart`, {
      headers: { "Authorization": `Bearer ${tokenA}` },
    })
    expect([401, 403]).toContain(res.status)
  })
})
