import { defineConfig, devices } from '@playwright/test'

const webPort = Number(process.env.E2E_WEB_PORT || 5173)
const webUrl = `http://localhost:${webPort}`

export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,        // tests share DB state; run serially
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  reporter: process.env.CI ? 'github' : 'list',
  // The first navigation cold-transforms the large match-page graph in Vite;
  // keep the suite from timing out mid-navigation on a clean dev server.
  timeout: 60_000,

  use: {
    baseURL: webUrl,
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },

  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
  ],

  // Start the dev server before running tests
  webServer: {
    command: `npm run dev -- --host 127.0.0.1 --port ${webPort}`,
    url: webUrl,
    reuseExistingServer: !process.env.CI && !process.env.E2E_WEB_PORT,
    timeout: 120_000,
  },
})
