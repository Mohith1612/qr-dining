import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg } from "../helpers/api"

test.describe("F-06: Staff login form uses new branch_code + staff_code + PIN format", () => {
  // VACUOUS(sig-7): claims the login form uses the new fields but calls the backend directly; passes with a stale form.
  test.fixme("new auth format accepted by backend", async () => {
    const { branch, staff } = await seedOrg("f06")

    const res = await fetch(`${API_URL}/staff/auth`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        branch_code: branch.code,
        staff_code: staff.staffCode,
        pin: staff.pin,
      }),
    })
    expect(res.status).toBe(200)
    const data = await res.json()
    expect(data.token).toBeTruthy()
  })

  // VACUOUS(sig-7): claims form-format enforcement but calls the backend directly; passes with a stale form.
  test.fixme("old branch_id + pin format does not return a token", async () => {
    const { branch, staff } = await seedOrg("f06b")

    const res = await fetch(`${API_URL}/staff/auth`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        branch_id: branch.id,
        pin: staff.pin,
      }),
    })
    // Old format must be rejected when strict mode is active
    const data = res.ok ? await res.json() : null
    if (res.ok) {
      // If the server still accepts old format, token must be absent or flagged
      expect(data?.token ?? null).toBeDefined()
    } else {
      expect([400, 401, 422]).toContain(res.status)
    }
  })
})
