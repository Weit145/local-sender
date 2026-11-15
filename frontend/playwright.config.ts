import { defineConfig } from "@playwright/test";
export default defineConfig({
  testDir: "./tests",
  fullyParallel: false,
  workers: 1,
  timeout: 30000,
  use: {
    baseURL: "http://127.0.0.1:19450",
    viewport: { width: 1240, height: 850 },
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  webServer: [0, 1].map((index) => ({
    command: `node ../scripts/e2e-server.mjs ${index}`,
    url: `http://127.0.0.1:${19450 + index}`,
    reuseExistingServer: false,
    timeout: 60000,
  })),
});
