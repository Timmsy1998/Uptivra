import { defineConfig } from "@playwright/test";
export default defineConfig({
  testDir: "./tests",
  testMatch: "demo.spec.ts",
  workers: 1,
  timeout: 30000,
  use: { baseURL: "http://localhost:3100", trace: "retain-on-failure" },
  webServer: {
    command: "npm run build && npm run start -- -p 3100",
    url: "http://localhost:3100",
    timeout: 180000,
    reuseExistingServer: false,
    env: { NEXT_PUBLIC_DEMO_MODE: "true", API_ORIGIN: "http://127.0.0.1:1" },
  },
});
