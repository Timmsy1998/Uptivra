import { test, expect } from "@playwright/test";

test("heartbeat creation, one-time token and authenticated receipt", async ({
  page,
  request,
}) => {
  await page.goto("/");
  await page
    .getByLabel("Email address")
    .fill(process.env.ADMIN_EMAIL || "browser@example.com");
  await page
    .getByLabel("Password", { exact: true })
    .fill(process.env.ADMIN_PASSWORD || "browser-test-password-only");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await page.getByRole("button", { name: "New monitor", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Monitor type").selectOption("heartbeat");
  await expect(dialog.getByLabel("Endpoint", { exact: true })).toHaveCount(0);
  await dialog.getByLabel("Display name").fill("Discord heartbeat test");
  await dialog.getByLabel("I own this service").check();
  await dialog.getByRole("button", { name: "Save monitor" }).click();
  await expect(dialog).not.toBeVisible();
  await page
    .locator(".service-link")
    .filter({ hasText: "Discord heartbeat test" })
    .click();
  await expect(
    page.getByRole("heading", { name: "Heartbeat connection" }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Response time", exact: true }),
  ).toHaveCount(0);
  page.on("dialog", (dialog) => dialog.accept());
  const generate = page.getByRole("button", {
    name: "Generate / replace heartbeat token",
  });
  await generate.click();
  const url = await page
    .getByLabel("Heartbeat URL", { exact: true })
    .inputValue();
  const tokenInput = page.getByLabel("Bearer token (save now; shown once)");
  const token = await tokenInput.inputValue();
  expect(token.length).toBeGreaterThan(32);
  expect((await request.post(url)).status()).toBe(401);
  expect(
    (
      await request.post(url, { headers: { Authorization: `Bearer ${token}` } })
    ).status(),
  ).toBe(204);
  await generate.click();
  await expect(tokenInput).not.toHaveValue(token);
  expect(
    (
      await request.post(url, { headers: { Authorization: `Bearer ${token}` } })
    ).status(),
  ).toBe(401);
  await page.getByRole("button", { name: "All monitors" }).click();
  await page
    .locator(".service-link")
    .filter({ hasText: "Discord heartbeat test" })
    .click();
  await expect(tokenInput).toHaveCount(0);
  const id = new URL(url).pathname.split("/").pop();
  expect(
    (
      await page.request.delete(`/api/v1/monitors/${id}`, {
        headers: { Origin: process.env.E2E_ORIGIN || "http://localhost:3000" },
      })
    ).status(),
  ).toBe(204);
});
