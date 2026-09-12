import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession } from "../helpers/api"

test.describe("R-03: WS ticket rate limiting", () => {
  test("exceeding ticket rate limit returns 429", async () => {
    const { table } = await seedOrg("r03")
    const created = await createSession(table.id, "RateLimitUser")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    // Issue many tickets rapidly (limit is 12/min per session)
    const attempts = await Promise.all(
      Array.from({ length: 15 }, () =>
        fetch(`${API_URL}/sessions/${sessionId}/ws-ticket`, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            "Authorization": `Bearer ${guestToken}`,
          },
          body: JSON.stringify({}),
        })
      )
    )

    const statuses = attempts.map((r) => r.status)
    expect(statuses.filter((status) => status === 201)).toHaveLength(12)
    expect(statuses.filter((status) => status === 429)).toHaveLength(3)
  })
})
