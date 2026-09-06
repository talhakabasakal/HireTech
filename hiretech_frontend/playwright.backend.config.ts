import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e-backend",
  timeout: 45_000,
  expect: { timeout: 10_000 },
  fullyParallel: false,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? "github" : "list",
  use: {
    baseURL: "http://127.0.0.1:3002",
    browserName: "chromium",
    channel: "chrome",
    headless: true,
    trace: "retain-on-failure",
  },
  webServer: {
    command: "NEXT_PUBLIC_DATA_MODE=api NEXT_PUBLIC_API_URL=http://127.0.0.1:8080 npm run dev:web -- --hostname 127.0.0.1 --port 3002",
    url: "http://127.0.0.1:3002/login",
    reuseExistingServer: false,
    timeout: 120_000,
  },
});
