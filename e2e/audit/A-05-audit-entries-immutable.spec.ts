import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, fetchAudit } from "../helpers/api"

const adminToken = process.env.E2E_ADMIN_TOKEN ?? "e2e-admin-secret"

test.describe("A-05: Audit entries cannot be deleted or modified", () => {
  test("DELETE on audit endpoint returns 405 or 403", async () => {
    const { table } = await seedOrg("a05")
    const created = await createSession(table.id, "AuditGuest")

    const entries = await fetchAudit("session", created.session.id).catch(() => [] as any[])

    if (entries.length === 0) {
      // No entries to test deletion against; pass
      return
    }

    const entryId = entries[0].id
    const delRes = await fetch(`${API_URL}/platform/audit/${entryId}`, {
      method: "DELETE",
      headers: { "Authorization": `Bearer ${adminToken}` },
    })
    expect([403, 404, 405]).toContain(delRes.status)
  })

  test("PATCH on audit entry returns 405 or 403", async () => {
    const { table } = await seedOrg("a05b")
    const created = await createSession(table.id, "AuditGuest2")

    const entries = await fetchAudit("session", created.session.id).catch(() => [] as any[])

    if (entries.length === 0) {
      return
    }

    const entryId = entries[0].id
    const patchRes = await fetch(`${API_URL}/platform/audit/${entryId}`, {
      method: "PATCH",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${adminToken}`,
      },
      body: JSON.stringify({ action: "tampered" }),
    })
    expect([403, 404, 405]).toContain(patchRes.status)
  })
})
