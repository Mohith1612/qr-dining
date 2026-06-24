import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"

const adminToken = process.env.E2E_ADMIN_TOKEN ?? "e2e-admin-secret"

test.describe("PT-04: Organization provisioning by platform admin", () => {
  test("platform admin creates org, branch, and table", async () => {
    const slug = `e2e-pt04-${Date.now()}`

    const orgRes = await fetch(`${API_URL}/platform/organizations`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${adminToken}`,
      },
      body: JSON.stringify({ name: "PT04 Org", slug }),
    })
    expect([200, 201]).toContain(orgRes.status)
    const org = await orgRes.json()
    expect(org.id).toBeTruthy()

    const branchRes = await fetch(`${API_URL}/platform/organizations/${org.id}/branches`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${adminToken}`,
      },
      body: JSON.stringify({ name: "Main Branch", code: slug }),
    })
    expect([200, 201]).toContain(branchRes.status)
    const branch = await branchRes.json()
    expect(branch.id).toBeTruthy()

    const tableRes = await fetch(`${API_URL}/branches/${branch.id}/tables`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${adminToken}`,
      },
      body: JSON.stringify({ identifier: "T1" }),
    })
    expect([200, 201]).toContain(tableRes.status)
  })
})
