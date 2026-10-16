import { test, expect } from "@playwright/test";

test("administrator workflow and redacted public status against real PostgreSQL", async ({
  page,
  request,
  browser,
}) => {
  const faults: string[] = [];
  page.on("pageerror", (e) => faults.push(e.message));
  await page.setViewportSize({ width: 1440, height: 960 });
  await page.goto("/");
  await expect(
    page.getByRole("heading", { name: "Sign in to Uptivra" }),
  ).toBeVisible();
  await page
    .getByLabel("Email address")
    .fill(process.env.ADMIN_EMAIL || "browser@example.com");
  await page.getByLabel("Password", { exact: true }).fill("incorrect-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page.getByRole("alert")).toBeVisible();
  await page
    .getByLabel("Password", { exact: true })
    .fill(process.env.ADMIN_PASSWORD || "browser-test-password-only");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Every endpoint. A clear picture." }),
  ).toBeVisible();
  await page.getByRole("button", { name: "New monitor", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await dialog.locator("[name=name]").fill("Example service");
  await dialog.locator("[name=group]").fill("Core services");
  await dialog.locator("[name=url]").fill("http://127.0.0.1");
  await dialog.locator("[name=authorized]").check();
  await dialog.getByRole("button", { name: "Save monitor" }).click();
  await expect(dialog.getByRole("alert")).toBeVisible();
  await dialog.locator("[name=url]").fill("https://example.com");
  await dialog.locator("[name=public]").check();
  await dialog.getByRole("button", { name: "Save monitor" }).click();
  await expect(dialog).not.toBeVisible();
  await expect(
    page.locator(".service-link").filter({ hasText: "Example service" }),
  ).toBeVisible();
  // Authenticated API shares the browser's HttpOnly session cookie.
  const monitors = await (await page.request.get("/api/v1/monitors")).json();
  const monitor = monitors.find(
    (m: { name: string }) => m.name === "Example service",
  );
  const privateResponse = await page.request.post("/api/v1/monitors", {
    headers: { Origin: process.env.E2E_ORIGIN || "http://localhost:3000" },
    data: {
      ...monitor,
      id: "",
      name: "Private internal service",
      url: "https://example.com/private-token",
      public: false,
      headers: { Authorization: "private-header-secret" },
    },
  });
  expect(privateResponse.status()).toBe(201);
  const privateID = (await privateResponse.json()).id;
  expect(
    process.env.CRON_SECRET,
    "Set CRON_SECRET to exercise actual outbound checks",
  ).toBeTruthy();
  const tick = await request.get(
    (process.env.API_ORIGIN || "http://localhost:8080") + "/api/cron",
    {
      headers: { Authorization: "Bearer " + process.env.CRON_SECRET },
      timeout: 60000,
    },
  );
  expect(
    tick.ok(),
    `Scheduler returned ${tick.status()}: ${await tick.text()}`,
  ).toBeTruthy();
  await page.getByRole("button", { name: "Refresh data" }).click();
  await expect(page.getByText("1 samples").first()).toBeVisible();
  await page
    .locator(".service-link")
    .filter({ hasText: "Example service" })
    .click();
  await expect(
    page.getByRole("heading", { name: "Recent checks" }),
  ).toBeVisible();
  await expect(
    page.getByRole("img", {
      name: "Response time of recent checks, ordered oldest to newest",
    }),
  ).toBeVisible();
  const history = await (
    await page.request.get("/api/v1/monitors/" + monitor.id + "/checks")
  ).json();
  expect(history[0].checked_at).toBeTruthy();
  await expect(
    page.locator("tbody tr").first().locator("td").first(),
  ).not.toHaveText("No checks yet");
  await expect(
    page.locator("tbody tr").first().locator("td").first(),
  ).not.toHaveText("Invalid Date");
  await page.getByRole("button", { name: "All monitors" }).click();
  await page.getByRole("button", { name: "Pause Example service" }).click();
  await expect(
    page.getByRole("button", { name: "Resume Example service" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Resume Example service" }).click();
  await expect(
    page.getByRole("button", { name: "Pause Example service" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Edit Example service" }).click();
  await dialog.locator("[name=description]").fill("Public example endpoint");
  await dialog.getByRole("button", { name: "Save monitor" }).click();
  await expect(dialog).not.toBeVisible();
  if (process.env.UPDATE_SCREENSHOTS)
    await page.screenshot({
      path: "../docs/screenshots/dashboard.png",
      fullPage: true,
    });
  await page.getByRole("button", { name: "Channels", exact: true }).click();
  await page.locator("[name=name]").fill("Browser webhook");
  await page.getByLabel("Destination URL").fill("https://example.com/webhook");
  await page
    .getByLabel("Signing secret")
    .fill("browser-test-signing-secret-not-production");
  await page.getByRole("button", { name: "Create channel" }).click();
  await expect(
    page.getByText("Channel created.", { exact: false }),
  ).toBeVisible();
  page.once("dialog", (d) => d.accept());
  await page.getByRole("button", { name: "Delete Browser webhook" }).click();
  await expect(
    page.getByRole("button", { name: "Delete Browser webhook" }),
  ).not.toBeVisible();
  await page
    .getByRole("button", { name: "Settings & access", exact: true })
    .click();
  await page.getByLabel("Page title").fill("Example operations");
  await page.getByRole("button", { name: "Save settings" }).click();
  await expect(page.getByText("Settings saved.")).toBeVisible();
  await page.getByLabel("Key name").fill("Browser reader");
  await page.getByRole("button", { name: "Create key" }).click();
  const token = await page.locator(".token code").innerText();
  const keyHeaders = { Authorization: "Bearer " + token };
  expect(
    (await request.get("/api/v1/monitors", { headers: keyHeaders })).status(),
  ).toBe(200);
  expect(
    (
      await request.delete("/api/v1/monitors/" + monitor.id, {
        headers: keyHeaders,
      })
    ).status(),
  ).toBe(403);
  page.once("dialog", (d) => d.accept());
  await page.getByRole("button", { name: "Revoke Browser reader" }).click();
  await expect(page.getByText("API key revoked.")).toBeVisible();
  expect(
    (await request.get("/api/v1/monitors", { headers: keyHeaders })).status(),
  ).toBe(401);
  const anonymous = await browser.newContext({
    viewport: { width: 390, height: 844 },
  });
  const publicPage = await anonymous.newPage();
  await publicPage.clock.install();
  await publicPage.goto(
    (process.env.E2E_ORIGIN || "http://localhost:3000") + "/status",
  );
  await expect(
    publicPage.getByRole("heading", { name: "Example operations" }),
  ).toBeVisible();
  await expect(
    publicPage.getByRole("heading", { name: "Example service" }),
  ).toBeVisible();
  const publicJSON = await (
    await publicPage.request.get(
      (process.env.E2E_ORIGIN || "http://localhost:3000") +
        "/api/v1/public/status",
    )
  ).text();
  for (const secret of [
    "Private internal service",
    "private-token",
    "private-header-secret",
    "https://example.com",
  ])
    expect(publicJSON).not.toContain(secret);
  expect(
    await publicPage.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBeTruthy();
  if (process.env.UPDATE_SCREENSHOTS)
    await publicPage.screenshot({
      path: "../docs/screenshots/status-mobile.png",
      fullPage: true,
    });
  await publicPage.bringToFront();
  await publicPage.route("**/api/v1/public/status", (route) => route.abort());
  await publicPage.clock.fastForward(30001);
  await expect(
    publicPage.getByRole("heading", { name: "Awaiting current service data" }),
  ).toBeVisible();
  await expect(
    publicPage.getByText("Current health unavailable.", { exact: false }),
  ).toBeVisible();
  await anonymous.close();
  await page.getByRole("button", { name: "Monitors", exact: true }).click();
  await page.setViewportSize({ width: 390, height: 844 });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBeTruthy();
  await page.getByRole("button", { name: "Toggle colour theme" }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  if (process.env.UPDATE_SCREENSHOTS)
    await page.screenshot({
      path: "../docs/screenshots/dashboard-mobile-dark.png",
      fullPage: true,
    });
  await page.setViewportSize({ width: 1440, height: 960 });
  page.once("dialog", (d) => d.accept());
  await page.getByRole("button", { name: "Delete Example service" }).click();
  await expect(
    page.getByRole("button", { name: "Edit Example service" }),
  ).not.toBeVisible();
  expect(
    (
      await page.request.delete("/api/v1/monitors/" + privateID, {
        headers: { Origin: process.env.E2E_ORIGIN || "http://localhost:3000" },
      })
    ).status(),
  ).toBe(204);
  await page.getByRole("button", { name: "Sign out" }).click();
  await expect(
    page.getByRole("heading", { name: "Sign in to Uptivra" }),
  ).toBeVisible();
  expect(faults).toEqual([]);
});

test("proxy rejects oversized request bodies", async ({ request }) => {
  const response = await request.post("/api/v1/monitors", {
    headers: { "Content-Type": "application/json" },
    data: " ".repeat(32769),
  });
  expect(response.status()).toBe(413);
});
