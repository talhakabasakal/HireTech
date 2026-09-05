import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  timeout: 30_000,
  expect: { timeout: 5_000 },
  fullyParallel: false,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 2 : 0,
  reporter: process.env.CI ? "github" : "list",
  use: {
    baseURL: "http://127.0.0.1:3001",
    browserName: "chromium",
    channel: "chrome",
    headless: true,
    trace: "retain-on-failure",
  },
  webServer: {
    command: "NEXT_PUBLIC_DATA_MODE=mock NEXT_PUBLIC_API_URL=http://127.0.0.1:8080 npm run dev:web -- --hostname 127.0.0.1 --port 3001",
    url: "http://127.0.0.1:3001/login",
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
});
