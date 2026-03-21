import { defineConfig, devices } from '@playwright/test'

const port = process.env.FRONTEND_TEST_PORT || '4173'
const baseURL = `http://127.0.0.1:${port}`

export default defineConfig({
  testDir: './tests/frontend',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  reporter: process.env.CI ? 'github' : 'list',
  use: {
    baseURL,
    trace: 'on-first-retry',
  },
  webServer: {
    command: 'go run ./cmd/frontendtestserver',
    url: `${baseURL}/health/live`,
    reuseExistingServer: false,
    timeout: 120000,
    env: {
      FRONTEND_TEST_PORT: port,
    },
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
})
