import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { getPlatformToken } from "../helpers/api"
import crypto from "crypto"

test.describe("PT-04: Organization provisioning by platform admin", () => {
  test("platform admin creates org, branch, and table", async () => {
    const suffix = crypto.randomUUID().slice(0, 8)
    const code = `e2e-pt04-${suffix}`
    const restaurantSlug = `e2e-pt04-restaurant-${suffix}`
    const platformToken = await getPlatformToken()

    const orgRes = await fetch(`${API_URL}/platform/organizations`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${platformToken}`,
      },
      body: JSON.stringify({
        code,
        name: "PT04 Org",
        restaurant_slug: restaurantSlug,
      }),
    })
    expect(orgRes.status).toBe(201)
    const orgResult = await orgRes.json() as {
      organization: { id: number; code: string }
      restaurant: { id: number; organization_id: number; slug: string }
    }
    expect(orgResult.organization.id).toBeGreaterThan(0)
    expect(orgResult.organization.code).toBe(code)
    expect(orgResult.restaurant.organization_id).toBe(orgResult.organization.id)
    expect(orgResult.restaurant.slug).toBe(restaurantSlug)

    const branchRes = await fetch(`${API_URL}/platform/organizations/${orgResult.organization.id}/branches`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${platformToken}`,
      },
      body: JSON.stringify({
        name: "Main Branch",
        branch_code: `PT04-${suffix}`,
        initial_tables: [{ identifier: "T1", capacity: 4 }],
      }),
    })
    expect(branchRes.status).toBe(201)
    const branchResult = await branchRes.json() as {
      branch: { id: number; organization_id: number; branch_code: string }
      tables: Array<{ id: number; branch_id: number; identifier: string; capacity: number }>
      initial_owner: null
    }
    expect(branchResult.branch.id).toBeGreaterThan(0)
    expect(branchResult.branch.organization_id).toBe(orgResult.organization.id)
    expect(branchResult.branch.branch_code).toBe(`PT04-${suffix}`.toUpperCase())
    expect(branchResult.tables).toHaveLength(1)
    expect(branchResult.tables[0].branch_id).toBe(branchResult.branch.id)
    expect(branchResult.tables[0].identifier).toBe("T1")
    expect(branchResult.tables[0].capacity).toBe(4)
    expect(branchResult.initial_owner).toBeNull()
  })
})
