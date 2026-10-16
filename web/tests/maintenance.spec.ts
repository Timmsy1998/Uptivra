import { test, expect } from "@playwright/test";
test("schedules weekly public maintenance and cancels it", async ({ page }) => {
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
    page.getByRole("button", { name: "Maintenance", exact: true }),
  ).toBeVisible();
  const name = "Maintenance service " + Date.now();
  const res = await page.request.post("/api/v1/monitors", {
    headers,
    data: {
      name,
      url: "https://example.com",
      interval_seconds: 60,
      timeout_seconds: 5,
      expected_status: 200,
      failure_threshold: 2,
      recovery_threshold: 2,
      enabled: true,
      public: true,
      authorized: true,
      headers: {},
    },
  });
  expect(res.status()).toBe(201);
  const monitors = await (await page.request.get("/api/v1/monitors")).json();
  const m = monitors.find((x: { name: string }) => x.name === name);
  const title = "Weekly upgrade " + Date.now();
  try {
    await page
      .getByRole("button", { name: "Maintenance", exact: true })
      .click();
    await page.getByLabel("Monitor", { exact: true }).selectOption(m.id);
    await page.getByLabel("Window title").fill(title);
    await page
      .getByLabel("Public description")
      .fill("Planned database upgrade");
    const local = (d: Date) =>
      new Date(d.getTime() - d.getTimezoneOffset() * 60000)
        .toISOString()
        .slice(0, 16);
    await page
      .getByLabel("Starts at (local time)")
      .fill(local(new Date(Date.now() + 3600000)));
    await page
      .getByLabel("Ends at (local time)")
      .fill(local(new Date(Date.now() + 7200000)));
    await page.getByLabel("Repeat weekly in UTC").check();
    await page
      .getByRole("button", { name: "Schedule maintenance", exact: true })
      .click();
    await expect(
      page.getByText("Maintenance scheduled.", { exact: true }),
    ).toBeVisible();
    await page.goto("/status");
    await expect(
      page.getByRole("heading", { name: title, exact: true }),
    ).toBeVisible();
    await page.goto("/");
    await page
      .getByRole("button", { name: "Maintenance", exact: true })
      .click();
    page.once("dialog", (d) => d.accept());
    await page
      .getByRole("button", { name: "Cancel " + title, exact: true })
      .click();
    await expect(
      page.getByText("Maintenance cancelled.", { exact: true }),
    ).toBeVisible();
    await page.goto("/status");
    await expect(
      page.getByRole("heading", { name: "Upcoming maintenance" }),
    ).toBeVisible();
    await expect(
      page.getByRole("heading", { name: title, exact: true }),
    ).toHaveCount(0);
  } finally {
    await page.request.delete("/api/v1/monitors/" + m.id, { headers });
  }
});
