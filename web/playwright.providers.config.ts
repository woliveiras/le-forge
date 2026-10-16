import { defineConfig, devices } from '@playwright/test';

const port = process.env.LEFORGE_PROVIDER_E2E_PORT ?? '18102';
export default defineConfig({
  testDir: './e2e-providers', workers: 1, retries: 0,
  timeout: 60000, expect: { timeout: 15000 },
  reporter: [['list']], outputDir: 'test-results-providers',
  use: { baseURL: `http://127.0.0.1:${port}`, actionTimeout: 15000,
    trace: 'retain-on-failure', screenshot: 'only-on-failure' },
  projects: [{ name: 'chromium-provider-langgraph', use: { ...devices['Desktop Chrome'] } }],
  webServer: {
    command: 'npm run build && cd ../server && go build -tags e2e -o bin/forge-provider-e2e ./cmd/forge-provider-e2e && exec bin/forge-provider-e2e',
    url: `http://127.0.0.1:${port}/healthz`, reuseExistingServer: false,
    timeout: 120000, gracefulShutdown: { signal: 'SIGTERM', timeout: 15000 },
  },
});
