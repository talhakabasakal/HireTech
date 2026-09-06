import { expect, test, type APIRequestContext } from "@playwright/test";

const backendURL = "http://127.0.0.1:8080";
const mailpitURL = "http://127.0.0.1:8025";
const email = "hiretech-browser-e2e@example.test";
const password = "Integration123!";

interface MailpitMessage {
  ID: string;
  Created: string;
  Snippet: string;
  To: Array<{ Address: string }>;
}

interface MailpitMessages { messages: MailpitMessage[] }

async function latestOtp(request: APIRequestContext, notBefore: number): Promise<string | null> {
  const response = await request.get(`${mailpitURL}/api/v1/messages`);
  if (!response.ok()) return null;
  const mailbox = await response.json() as MailpitMessages;
  const message = mailbox.messages.find((item) =>
    item.To.some((recipient) => recipient.Address === email) && Date.parse(item.Created) >= notBefore - 1_000,
  );
  return message?.Snippet.match(/\b\d{6}\b/)?.[0] ?? null;
}

test("authenticates through the real backend and Mailpit OTP", async ({ page, request }) => {
  const setup = await request.post(`${backendURL}/api/v1/auth/register`, {
    data: { email, password, first_name: "Browser", last_name: "E2E" },
  });
  expect([200, 201, 409]).toContain(setup.status());

  const observed = new Map<string, number>();
  page.on("response", (response) => {
    const url = new URL(response.url());
    if (url.origin === backendURL) observed.set(url.pathname, response.status());
  });

  await page.goto("/login");
  await page.getByLabel("Work email", { exact: true }).fill(email);
  await page.getByLabel("Password", { exact: true }).fill(password);
  const otpRequestedAfter = Date.now();
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page).toHaveURL(/\/verify\?email=hiretech-browser-e2e%40example\.test/);

  let otp: string | null = null;
  await expect.poll(async () => {
    otp = await latestOtp(request, otpRequestedAfter);
    return otp;
  }, { timeout: 10_000 }).toMatch(/^\d{6}$/);

  for (const [index, digit] of [...otp!].entries()) {
    await page.getByLabel(`Digit ${index + 1}`, { exact: true }).fill(digit);
  }
  await page.getByRole("button", { name: "Verify identity", exact: true }).click();
  await expect(page).toHaveURL(/\/candidate$/);

  expect(observed.get("/api/v1/auth/login")).toBe(200);
  expect(observed.get("/api/v1/auth/otp/request")).toBe(200);
  expect(observed.get("/api/v1/auth/otp/verify")).toBe(200);
  expect(observed.get("/api/v1/me")).toBe(200);
  await expect(page.getByText("Demo mode — isolated UI data only.")).toHaveCount(0);
});
