import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: 'html',
  use: {
    baseURL: 'http://localhost:7827',
    trace: 'on-first-retry',
  },
  projects: [
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
        // AIDEV-NOTE: Default to Playwright's bundled chromium (correct in CI).
        // On hosts where that binary can't link the system libs (e.g. NixOS),
        // point PLAYWRIGHT_CHROMIUM_BIN at a working Chrome/Chromium executable.
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
