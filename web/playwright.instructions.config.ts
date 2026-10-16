import { defineConfig, devices } from "@playwright/test";
export default defineConfig({
  testDir: "./e2e",
  testMatch: "instructions.spec.ts",
  workers: 1,
  retries: 0,
  timeout: 60000,
  expect: { timeout: 15000 },
  reporter: "list",
  use: {
    baseURL: "http://127.0.0.1:18104",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: {
    command:
      "npm run build && cd ../server && go build -tags e2e -o bin/forge-instructions-e2e ./cmd/forge-e2e && LEFORGE_E2E_ADDR=127.0.0.1:18104 exec bin/forge-instructions-e2e",
    url: "http://127.0.0.1:18104/healthz",
    reuseExistingServer: false,
    timeout: 120000,
    gracefulShutdown: { signal: "SIGTERM", timeout: 15000 },
  },
});
