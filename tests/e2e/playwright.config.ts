import { defineConfig, devices } from '@playwright/test';
import { getConfig } from './lib/config';

// Validate ALL required env vars upfront - fail fast before any tests run
const config = getConfig();

export default defineConfig({
  testDir: './tests',
  fullyParallel: false, // Run sequentially - EPX sandbox may have rate limits
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: 1, // Single worker for E2E tests hitting real EPX
  reporter: 'html',

  timeout: 60000, // 60 seconds per test - EPX can be slow

  use: {
    baseURL: config.serviceURL,
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },

  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],

  // Run local server before tests if needed
  // webServer: {
  //   command: 'podman-compose up -d',
  //   url: 'http://localhost:8081/cron/health',
  //   reuseExistingServer: !process.env.CI,
  // },
});
