import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, loginStaff } from "../helpers/api"

const adminToken = process.env.E2E_ADMIN_TOKEN ?? "e2e-admin-secret"

test.describe("S-06: Role separation — waiter cannot access admin endpoints", () => {
  test("waiter token rejected on branch admin operations", async () => {
    const { branch, staff } = await seedOrg("s06")

    const waiterCtx = await loginStaff(branch.code, staff.staffCode, staff.pin)
    expect(waiterCtx.role).toBe("waiter")

    // Waiter tries to create another staff member — should be forbidden
    const res = await fetch(`${API_URL}/branches/${branch.id}/staff`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${waiterCtx.token}`,
      },
      body: JSON.stringify({
        name: "Unauthorized Staff",
        role: "waiter",
        staff_code: `unauth-${Date.now()}`,
        pin: "111111",
      }),
    })
    expect([401, 403]).toContain(res.status)
  })

  test("waiter token rejected on menu management", async () => {
    const { branch, staff, menu } = await seedOrg("s06b")

    const waiterCtx = await loginStaff(branch.code, staff.staffCode, staff.pin)

    // Waiter tries to update menu item price
    const res = await fetch(`${API_URL}/branches/${branch.id}/menu/items/${menu.itemId}`, {
      method: "PATCH",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${waiterCtx.token}`,
      },
      body: JSON.stringify({ price: 1 }),
    })
    expect([401, 403]).toContain(res.status)
  })
})

// Regression: both endpoints below relied solely on the central authorizer,
// which runs in shadow mode by default (AUTHZ_CENTRAL_POLICY_ENFORCE=false) and
// logs-and-allows on denial. A waiter could therefore read the roster to find
// the owner's id and then overwrite the owner's PIN — no current PIN required —
// and log straight back in as owner. These tests MUST run with the authorizer
// at its shipped default so they exercise the fail-open path.
test.describe("S-06c: Staff PIN reset and roster are owner/manager only", () => {
  async function seedOwnerAndWaiter(label: string) {
    const { branch, staff } = await seedOrg(label)
    const ownerCtx = await loginStaff(branch.code, staff.staffCode, staff.pin)

    const waiterCode = `waiter-${Date.now()}`
    const waiterPin = "246810"
    const createRes = await fetch(`${API_URL}/branches/${branch.id}/staff`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${ownerCtx.token}`,
      },
      body: JSON.stringify({
        name: "Waiter Wanda",
        role: "waiter",
        staff_code: waiterCode,
        pin: waiterPin,
      }),
    })
    expect(createRes.status).toBe(201)

    const waiterCtx = await loginStaff(branch.code, waiterCode, waiterPin)
    expect(waiterCtx.role).toBe("waiter")
    return { branch, staff, ownerCtx, waiterCtx }
  }

  test("waiter cannot reset the owner's PIN", async () => {
    const { branch, staff, ownerCtx, waiterCtx } = await seedOwnerAndWaiter("s06c")

    const attackerPin = "913746"
    const res = await fetch(`${API_URL}/staff/${ownerCtx.staffId}/pin/reset`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${waiterCtx.token}`,
      },
      body: JSON.stringify({ new_pin: attackerPin }),
    })
    expect(res.status).toBe(403)

    // The attacker's chosen PIN must NOT authenticate as the owner.
    const takeover = await fetch(`${API_URL}/staff/auth`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        branch_code: branch.code,
        staff_code: staff.staffCode,
        pin: attackerPin,
      }),
    })
    expect(takeover.status).not.toBe(200)

    // ...and the owner's real PIN must still work, proving nothing was written.
    const owner = await loginStaff(branch.code, staff.staffCode, staff.pin)
    expect(owner.role).toBe("owner")
  })

  test("waiter cannot read the staff roster; owner can", async () => {
    const { branch, ownerCtx, waiterCtx } = await seedOwnerAndWaiter("s06d")

    const waiterRes = await fetch(`${API_URL}/branches/${branch.id}/staff`, {
      headers: { "Authorization": `Bearer ${waiterCtx.token}` },
    })
    expect(waiterRes.status).toBe(403)

    const ownerRes = await fetch(`${API_URL}/branches/${branch.id}/staff`, {
      headers: { "Authorization": `Bearer ${ownerCtx.token}` },
    })
    expect(ownerRes.status).toBe(200)
  })
})
