import { test, expect } from "@playwright/test";
test("shows real empty coverage for all periods in workspace and public history", async ({
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
    page.getByRole("button", { name: "Maintenance", exact: true }),
  ).toBeVisible();
  const name = "History service " + Date.now();
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
  try {
    await page.reload();
    await page.getByText(name, { exact: true }).click();
    const panel = page.getByRole("region", { name: "Availability history" });
    await expect(
      panel.getByText("No measured coverage in this period.", { exact: true }),
    ).toBeVisible();
    for (const days of [7, 30, 90]) {
      await panel.getByLabel("History period").selectOption(String(days));
      await expect(panel.locator(".availability-day")).toHaveCount(days);
      await expect(
        panel.getByText("No measured coverage in this period.", {
          exact: true,
        }),
      ).toBeVisible();
    }
    await page.goto("/status");
    await page.getByLabel("Service history").selectOption(m.id);
    await panel.getByLabel("History period").selectOption("90");
    await expect(panel.locator(".availability-day")).toHaveCount(90);
    await panel.locator(".availability-day").first().focus();
    await expect(panel.locator(".availability-day").first()).toBeFocused();
    await page.setViewportSize({ width: 390, height: 844 });
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    ).toBe(true);
    await page.emulateMedia({ colorScheme: "dark" });
    await expect(
      panel.getByRole("heading", { name: "Incident durations" }),
    ).toBeVisible();
  } finally {
    await page.request.delete("/api/v1/monitors/" + m.id, { headers });
  }
});
