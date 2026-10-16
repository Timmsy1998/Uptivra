import { test, expect } from "@playwright/test";
test("queues channel tests and displays scheduler and delivery diagnostics", async ({
  page,
}) => {
  const headers = { Origin: process.env.E2E_ORIGIN || "http://localhost:3000" };
  await page.goto("/");
  await page
    .getByLabel("Email address")
    .fill(process.env.ADMIN_EMAIL || "browser@example.com");
  await page
    .getByLabel("Password", { exact: true })
    .fill(process.env.ADMIN_PASSWORD || "browser-test-password-only");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "Channels", exact: true }),
  ).toBeVisible();
  const name = "Diagnostic " + Date.now();
  const created = await page.request.post("/api/v1/channels", {
    headers,
    data: {
      name,
      kind: "webhook",
      url: "https://example.com/test",
      signing_secret: "test-signing-secret-for-browser-only",
    },
  });
  expect(created.status()).toBe(201);
  const channels = await (await page.request.get("/api/v1/channels")).json();
  const channel = channels.find((c: { name: string }) => c.name === name);
  try {
    await page.getByRole("button", { name: "Channels", exact: true }).click();
    await page
      .getByRole("button", { name: "Test " + name, exact: true })
      .click();
    await expect(page.getByText(/Test queued/)).toBeVisible();
    await page
      .getByRole("button", { name: "Delivery & scheduler", exact: true })
      .click();
    await expect(
      page.getByRole("heading", { name: "Scheduler health" }),
    ).toBeVisible();
    await expect(page.getByRole("cell", { name, exact: false })).toBeVisible();
    await page.getByLabel("Delivery state").selectOption("failed");
    await expect(page.getByRole("cell", { name, exact: false })).toHaveCount(0);
  } finally {
    await page.request.delete("/api/v1/channels/" + channel.id, { headers });
  }
});
