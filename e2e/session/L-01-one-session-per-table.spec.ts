import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg } from "../helpers/api"

test.describe("L-01: One session per table under concurrency", () => {
  test("5 concurrent creates on same table → exactly 1 success, 4 conflicts", async () => {
    const { table } = await seedOrg("l01")

    const attempts = await Promise.all(
      Array.from({ length: 5 }, (_, i) =>
        fetch(`${API_URL}/sessions`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ table_id: table.id, display_name: `Guest${i}` }),
        })
      )
    )

    const statuses = attempts.map((r) => r.status)
    const successes = statuses.filter((s) => s === 201)
    const conflicts = statuses.filter((s) => s === 409)

    expect(successes.length).toBe(1)
    expect(conflicts.length).toBe(4)
  })
})
