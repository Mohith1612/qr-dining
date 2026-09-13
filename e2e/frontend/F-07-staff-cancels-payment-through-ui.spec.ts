import { test, expect } from "@playwright/test"
import { API_URL, BASE_URL } from "../playwright.config"
import { seedOrg, createSession, placeOrder, fetchAudit } from "../helpers/api"
import crypto from "crypto"

/**
 * F-07: A waiter cancels a bill request from the staff UI.
 *
 * Drives the real browser, not the API: the backend route is already covered by
 * handlers/payment_cancel_integration_test.go, so what needs proving here is
 * that the UI reaches it — the affordance is on the waiter's own board, the
 * mandatory reason is enforced before anything is sent, and the session really
 * does unfreeze for the guest afterwards.
 */
test.describe("F-07: Waiter cancels a payment through the staff UI", () => {
  test("reason is mandatory, cancel unfreezes the table, guest can order again", async ({ page }) => {
    const { branch, table, menu, staff } = await seedOrg("f07", { staffRole: "waiter" })

    // A guest orders and asks for the bill, which freezes the session.
    const created = await createSession(table.id, "Priya")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token
    await placeOrder(sessionId, guestToken, menu.itemId, 1)

    const payRes = await fetch(`${API_URL}/sessions/${sessionId}/payments`, {
      method: "POST",
      headers: { "Content-Type": "application/json", Authorization: `Bearer ${guestToken}` },
      body: JSON.stringify({
        amount: menu.itemPrice,
        method: "cash",
        idempotency_key: crypto.randomUUID(),
      }),
    })
    expect(payRes.status).toBe(201)

    // The freeze is real before we touch the UI: the cart refuses writes.
    const frozenCart = await fetch(`${API_URL}/sessions/${sessionId}/cart/items`, {
      method: "POST",
      headers: { "Content-Type": "application/json", Authorization: `Bearer ${guestToken}` },
      body: JSON.stringify({ menu_item_id: menu.itemId, quantity: 1 }),
    })
    expect(frozenCart.status).toBe(409)

    // Record every cancel attempt so "no request without a reason" is provable.
    const cancelRequests: string[] = []
    page.on("request", (req) => {
      if (/\/payments\/\d+\/cancel$/.test(new URL(req.url()).pathname)) {
        cancelRequests.push(req.method())
      }
    })

    // Sign in as the waiter through the real login form.
    await page.goto(`${BASE_URL}/staff/login`)
    await page.getByPlaceholder("main-restaurant").fill(branch.code)
    await page.getByPlaceholder("your-staff-code").fill(staff.staffCode)
    await page.getByLabel("PIN").fill(staff.pin)
    await page.getByRole("button", { name: "Sign in" }).click()
    await expect(page).toHaveURL(/\/staff\/waiter/, { timeout: 10_000 })

    // The bill request is on the waiter's own board, with the recovery action.
    const cancelButton = page.getByRole("button", { name: "Cancel request" })
    await expect(cancelButton).toBeVisible({ timeout: 10_000 })
    await cancelButton.click()

    const sheet = page.getByRole("dialog")
    await expect(sheet).toBeVisible()
    const submit = sheet.getByRole("button", { name: "Cancel bill request" })

    // An empty reason is refused client-side — nothing is sent.
    await expect(submit).toBeDisabled()
    // Whitespace is not a reason either.
    await sheet.getByRole("textbox", { name: "Reason (required)" }).fill("    ")
    await expect(submit).toBeDisabled()
    expect(cancelRequests).toEqual([])

    const reason = "Guest changed their mind and wants to keep ordering"
    await sheet.getByRole("textbox", { name: "Reason (required)" }).fill(reason)
    await expect(submit).toBeEnabled()
    await submit.click()

    // The card leaves the queue and the waiter is told what happened.
    await expect(page.getByText("Bill request cancelled", { exact: false })).toBeVisible({ timeout: 10_000 })
    await expect(cancelButton).toHaveCount(0)
    expect(cancelRequests).toEqual(["PATCH"])

    // The session is unfrozen server-side and the guest can add an item again.
    const reopenedCart = await fetch(`${API_URL}/sessions/${sessionId}/cart/items`, {
      method: "POST",
      headers: { "Content-Type": "application/json", Authorization: `Bearer ${guestToken}` },
      body: JSON.stringify({ menu_item_id: menu.itemId, quantity: 1 }),
    })
    expect(reopenedCart.status).toBe(201)

    // The reason the waiter typed is what reached the audit trail.
    const audit = await fetchAudit("session", sessionId)
    const cancelEntry = audit.find((a) => a.action === "payment.cancel")
    expect(cancelEntry).toBeDefined()
    expect(JSON.stringify(cancelEntry)).toContain(reason)
  })
})
