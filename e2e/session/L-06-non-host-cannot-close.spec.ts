import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession } from "../helpers/api"

test.describe("L-06: Non-host cannot close session", () => {
  test("non-host DELETE returns 403", async () => {
    const { table } = await seedOrg("l06")
    const hostCreated = await createSession(table.id, "Host")
    const sessionId = hostCreated.session.id

    const joinRes = await fetch(`${API_URL}/sessions/${sessionId}/join`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ display_name: "NonHost" }),
    })
    expect(joinRes.status).toBe(201)
    const joinData = await joinRes.json()
    const nonHostToken = joinData.guest_access_token

    const closeRes = await fetch(`${API_URL}/sessions/${sessionId}`, {
      method: "DELETE",
      headers: { "Authorization": `Bearer ${nonHostToken}` },
    })
    expect(closeRes.status).toBe(403)
  })
})
