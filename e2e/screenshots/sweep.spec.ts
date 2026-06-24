import { test, expect } from "@playwright/test"
import { BASE_URL, API_URL } from "../playwright.config"
import { seedOrg, createSession, placeOrder, loginStaff } from "../helpers/api"
import path from "path"
import fs from "fs"

function screenshotDir(viewport: string, role: string): string {
  const dir = path.join(__dirname, viewport, role)
  fs.mkdirSync(dir, { recursive: true })
  return dir
}

function viewportLabel(width: number): string {
  if (width <= 375) return "mobile"
  if (width <= 768) return "tablet"
  return "desktop"
}

test.describe("Screenshot sweep — all viewports and roles", () => {
  test("guest flow: QR landing → session → menu → cart → payment", async ({ page, viewport }) => {
    const vp = viewportLabel(viewport?.width ?? 1280)
    const dir = screenshotDir(vp, "guest")

    await page.goto(`${BASE_URL}/`)
    await page.screenshot({ path: path.join(dir, "01-landing.png"), fullPage: true })

    // Seed and navigate to a session
    const { table, menu } = await seedOrg(`sweep-guest-${vp}-${Date.now()}`)
    const created = await createSession(table.id, "SweepGuest")
    const sessionId = created.session.id

    await page.goto(`${BASE_URL}/session/${sessionId}`)
    await page.waitForTimeout(1000)
    await page.screenshot({ path: path.join(dir, "02-session-dashboard.png"), fullPage: true })

    await page.goto(`${BASE_URL}/session/${sessionId}/menu`)
    await page.waitForTimeout(1000)
    await page.screenshot({ path: path.join(dir, "03-menu.png"), fullPage: true })

    await page.goto(`${BASE_URL}/session/${sessionId}/cart`)
    await page.waitForTimeout(1000)
    await page.screenshot({ path: path.join(dir, "04-cart.png"), fullPage: true })

    await page.goto(`${BASE_URL}/session/${sessionId}/payment`)
    await page.waitForTimeout(1000)
    await page.screenshot({ path: path.join(dir, "05-payment.png"), fullPage: true })
  })

  test("staff flow: login → dashboard", async ({ page, viewport }) => {
    const vp = viewportLabel(viewport?.width ?? 1280)
    const dir = screenshotDir(vp, "staff")

    await page.goto(`${BASE_URL}/staff/login`)
    await page.waitForTimeout(500)
    await page.screenshot({ path: path.join(dir, "01-staff-login.png"), fullPage: true })

    const { branch, staff } = await seedOrg(`sweep-staff-${vp}-${Date.now()}`)

    await page.fill("input[name='branchCode'], input[placeholder*='ranch']", branch.code).catch(() => {})
    await page.fill("input[name='staffCode'], input[placeholder*='taff']", staff.staffCode).catch(() => {})
    await page.fill("input[name='pin'], input[type='password']", staff.pin).catch(() => {})
    await page.screenshot({ path: path.join(dir, "02-staff-login-filled.png"), fullPage: true })
  })

  test("session ended screen", async ({ page, viewport }) => {
    const vp = viewportLabel(viewport?.width ?? 1280)
    const dir = screenshotDir(vp, "guest")

    const { table } = await seedOrg(`sweep-closed-${vp}-${Date.now()}`)
    const created = await createSession(table.id, "ClosedGuest")
    const sessionId = created.session.id

    // Force close via API
    await fetch(`${API_URL}/platform/sessions/${sessionId}/force-close`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${process.env.E2E_ADMIN_TOKEN ?? "e2e-admin-secret"}`,
      },
      body: JSON.stringify({ reason: "screenshot_sweep" }),
    })

    await page.goto(`${BASE_URL}/session/${sessionId}`)
    await page.waitForTimeout(1500)
    await page.screenshot({ path: path.join(dir, "06-session-ended.png"), fullPage: true })
  })

  test("payment pending screen", async ({ page, viewport }) => {
    const vp = viewportLabel(viewport?.width ?? 1280)
    const dir = screenshotDir(vp, "guest")

    const { table, menu } = await seedOrg(`sweep-pay-${vp}-${Date.now()}`)
    const created = await createSession(table.id, "PayGuest")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    await placeOrder(sessionId, guestToken, menu.itemId, 1)

    await fetch(`${API_URL}/sessions/${sessionId}/payments`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${guestToken}`,
      },
      body: JSON.stringify({
        amount: menu.itemPrice,
        method: "cash",
        idempotency_key: `sweep-${Date.now()}`,
      }),
    })

    await page.goto(`${BASE_URL}/session/${sessionId}/payment`)
    await page.waitForTimeout(1000)
    await page.screenshot({ path: path.join(dir, "07-payment-pending.png"), fullPage: true })
  })

  test("platform admin login page", async ({ page, viewport }) => {
    const vp = viewportLabel(viewport?.width ?? 1280)
    const dir = screenshotDir(vp, "platform-admin")

    await page.goto(`${BASE_URL}/platform`)
    await page.waitForTimeout(500)
    await page.screenshot({ path: path.join(dir, "01-platform-login.png"), fullPage: true })
  })
})
