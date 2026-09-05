import { expect, test } from "@playwright/test";

test("signs in through OTP and reaches the candidate invitation", async ({ page }) => {
  await page.goto("/login");
  await expect(page.getByRole("heading", { name: "Sign in to HireTech" })).toBeVisible();

  await page.getByLabel("Work email", { exact: true }).fill("smoke@example.test");
  await page.getByLabel("Password", { exact: true }).fill("demo-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page).toHaveURL(/\/verify\?email=smoke%40example\.test/);

  for (const [index, digit] of [..."123456"].entries()) {
    await page.getByLabel(`Digit ${index + 1}`, { exact: true }).fill(digit);
  }
  await page.getByRole("button", { name: "Verify identity", exact: true }).click();

  await expect(page).toHaveURL(/\/candidate$/);
  await expect(page.getByRole("heading", { name: "Show us how you think." })).toBeVisible();
  await page.getByRole("link", { name: "Enter candidate area", exact: true }).click();

  await expect(page).toHaveURL(/\/candidate\/invitation$/);
  await expect(page.getByRole("heading", { name: "Your technical interview" })).toBeVisible();
  await expect(page.getByText("Demo mode — isolated UI data only.")).toBeVisible();
});
