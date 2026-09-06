import { defineConfig } from "@playwright/test";

const browserChannel = process.env.HIRETECH_E2E_BROWSER_CHANNEL ?? "chrome";

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
    ...(browserChannel === "none" ? {} : { channel: browserChannel }),
    headless: true,
    trace: "off",
    screenshot: "off",
    video: "off",
  },
  webServer: {
    command: `NEXT_PUBLIC_DATA_MODE=api NEXT_PUBLIC_API_URL=${process.env.HIRETECH_E2E_BACKEND_URL ?? "http://127.0.0.1:8080"} npm run dev:web -- --hostname 127.0.0.1 --port 3002`,
    url: "http://127.0.0.1:3002/login",
    reuseExistingServer: false,
    timeout: 120_000,
  },
});
