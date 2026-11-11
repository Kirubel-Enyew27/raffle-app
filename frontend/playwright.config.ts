import { defineConfig, devices } from '@playwright/test'

const webPort = Number(process.env.E2E_WEB_PORT || 5173)
const webUrl = `http://localhost:${webPort}`
// Production-build validation starts `vite preview` itself so Playwright does
// not race it with a dev server.
const externalWebServer = !!process.env.E2E_EXTERNAL_WEB_SERVER

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
    screenshot: 'only-on-failure',    // Two-session latency tests keep one page hidden in its context. Disable
    // renderer backgrounding so a measured rAF is the real paint frame, not a
    // throttled background tick.
    launchOptions: {
      args: [
        '--disable-background-timer-throttling',
        '--disable-backgrounding-occluded-windows',
        '--disable-renderer-backgrounding',
      ],
    },
  },

  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
  ],

  // Start the dev server before running tests unless an external production
  // preview was supplied.
  webServer: externalWebServer
    ? undefined
    : {
        command: `npm run dev -- --host 127.0.0.1 --port ${webPort}`,
        url: webUrl,
        reuseExistingServer: !process.env.CI && !process.env.E2E_WEB_PORT,
        timeout: 120_000,
      },
})
