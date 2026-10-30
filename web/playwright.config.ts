import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './e2e', timeout: 45000, workers: 1, fullyParallel: false,
  use: { baseURL: 'http://127.0.0.1:18880', headless: true, viewport: { width: 1440, height: 1000 }, trace: 'retain-on-failure', screenshot: 'only-on-failure' },
  webServer: { command: 'node e2e/server.mjs', url: 'http://127.0.0.1:18880/api/status', reuseExistingServer: false, timeout: 30000 },
})
