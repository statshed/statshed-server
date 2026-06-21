import { defineConfig, devices } from '@playwright/test'

// AIDEV-NOTE: NON-MOCKED live config (Task 5.5, spec §7.4). Distinct from the hermetic e2e/
// suite: it loads the app through the Vite dev proxy (:7827 -> the REAL Go server at :7828),
// so streaming and reconnect actually traverse the proxy. The Go server is spawned/stopped
// INSIDE the spec (the reconnect test restarts it), so it is NOT a webServer here — only
// Vite is. `make live-e2e` builds the Go binary and points GO_BIN at it.
export default defineConfig({
  testDir: './e2e-live',
  fullyParallel: false, // the reconnect test stops/starts Go; specs must run serially
  workers: 1,
  forbidOnly: !!process.env.CI,
  retries: 0,
  reporter: 'line',
  timeout: 60_000,
  use: {
    baseURL: 'http://localhost:7827',
    trace: 'on-first-retry',
  },
  projects: [
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
        // AIDEV-NOTE: Bundled chromium in CI; on hosts where it can't link the system libs
        // (e.g. NixOS) set PLAYWRIGHT_CHROMIUM_BIN to a working Chrome/Chromium.
        launchOptions: {
          executablePath: process.env.PLAYWRIGHT_CHROMIUM_BIN || undefined,
        },
      },
    },
  ],
  webServer: {
    command: 'npm run dev',
    url: 'http://localhost:7827',
    reuseExistingServer: !process.env.CI,
  },
})
