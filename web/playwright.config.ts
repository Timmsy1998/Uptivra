import { defineConfig } from "@playwright/test";
export default defineConfig({
  testDir: "./tests",
  workers: 1,
  timeout: 90000,
  retries: 0,
  use: {
    baseURL: process.env.E2E_ORIGIN || "http://localhost:3000",
    trace: "retain-on-failure",
  },
  reporter: "list",
});
