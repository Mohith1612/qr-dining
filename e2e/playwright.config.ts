import { defineConfig, devices } from "@playwright/test"

const API_URL = process.env.API_URL ?? "http://localhost:8090"
const APP_URL = process.env.APP_URL ?? "http://localhost:3000"

export default defineConfig({
  testDir: ".",
  globalSetup: require.resolve("./global-setup"),
  fullyParallel: true,
  retries: process.env.CI ? 2 : 0,
  // The shipped manual stack is one backend and one database. Phase 1 verified
  // two workers; callers can still override this explicitly on the CLI.
  workers: 2,
  reporter: [["list"], ["html", { outputFolder: "artifacts/report", open: "never" }]],
  outputDir: "artifacts/results",
  timeout: 30_000,

  use: {
    baseURL: APP_URL,
    screenshot: "only-on-failure",
    video: "retain-on-failure",
    trace: "retain-on-failure",
  },

  projects: [
    {
      name: "desktop",
      use: {
        ...devices["Desktop Chrome"],
        viewport: { width: 1280, height: 800 },
      },
    },
    {
      name: "mobile",
      use: {
        ...devices["iPhone 12"],
        viewport: { width: 375, height: 667 },
      },
    },
    {
      name: "tablet",
      use: {
        ...devices["iPad Mini"],
        viewport: { width: 768, height: 1024 },
      },
    },
  ],
})

export { API_URL, APP_URL }
export const BASE_URL = APP_URL
