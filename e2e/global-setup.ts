import type { FullConfig } from "@playwright/test"

const API_URL = process.env.API_URL ?? "http://localhost:8090"
const APP_URL = process.env.APP_URL ?? "http://localhost:3000"
const platformEmail = process.env.E2E_PLATFORM_EMAIL ?? "admin@platform.local"
const platformPassword = process.env.E2E_PLATFORM_PASSWORD ?? "Platform!admin1"

async function requireStatus(label: string, response: Response, expectedStatus: number): Promise<void> {
  if (response.status === expectedStatus) return
  const body = await response.text()
  throw new Error(`${label} returned ${response.status}; expected ${expectedStatus}: ${body}`)
}

export default async function globalSetup(_config: FullConfig): Promise<void> {
  const [apiReady, appReady] = await Promise.all([
    fetch(`${API_URL}/readyz`),
    fetch(`${APP_URL}/staff/login`),
  ])
  await requireStatus(`API preflight ${API_URL}/readyz`, apiReady, 200)
  await requireStatus(`App preflight ${APP_URL}/staff/login`, appReady, 200)

  const authResponse = await fetch(`${API_URL}/platform/auth`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      email: platformEmail,
      password: platformPassword,
      device_name: "playwright-global-setup",
    }),
  })
  await requireStatus("E2E platform login", authResponse, 200)

  const auth = await authResponse.json() as {
    token?: string
    roles?: string[]
    mfa_required?: boolean
  }
  if (auth.mfa_required) {
    throw new Error(
      "E2E platform account requires MFA; configure E2E_PLATFORM_EMAIL/E2E_PLATFORM_PASSWORD for a non-enrolled super_admin test account"
    )
  }
  if (!auth.token || !auth.roles?.includes("super_admin")) {
    throw new Error("E2E platform login did not return a super_admin session")
  }

  // Playwright worker processes inherit this real, short-lived server session.
  process.env.E2E_PLATFORM_TOKEN = auth.token
  console.log(`[e2e preflight] API_URL=${API_URL} APP_URL=${APP_URL} platform_auth=super_admin`)
}
