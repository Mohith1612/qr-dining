import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, loginStaff, fetchAudit } from "../helpers/api"

test.describe("A-03: Staff authentication events audited", () => {
  // VACUOUS(sig-1): audit fetch failures become an empty array; passes when successful logins are never audited.
  test.fixme("successful staff login appears in audit log", async () => {
    const { branch, staff } = await seedOrg("a03")
    const staffCtx = await loginStaff(branch.code, staff.staffCode, staff.pin)

    const entries = await fetchAudit("staff", String(staffCtx.staffId)).catch(() => [] as any[])

    if (entries.length > 0) {
      const actions = entries.map((e: any) => e.action ?? e.event_type ?? e.type)
      const hasAuth = actions.some((a: string) =>
        a?.toLowerCase().includes("auth") || a?.toLowerCase().includes("login")
      )
      expect(typeof hasAuth).toBe("boolean")
    }
  })

  // VACUOUS(sig-1): audit fetch failures become an empty array; passes when failed logins are never audited.
  test.fixme("failed staff login attempt is audit-logged", async () => {
    const { branch, staff } = await seedOrg("a03b")

    await fetch(`${API_URL}/staff/auth`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        branch_code: branch.code,
        staff_code: staff.staffCode,
        pin: "000000",
      }),
    })

    const entries = await fetchAudit("staff", String(staff.id)).catch(() => [] as any[])

    if (entries.length > 0) {
      const hasFailed = entries.some((e: any) => {
        const a = e.action ?? e.event_type ?? e.type ?? ""
        return a.toLowerCase().includes("fail") || a.toLowerCase().includes("denied")
      })
      expect(typeof hasFailed).toBe("boolean")
    }
  })
})
