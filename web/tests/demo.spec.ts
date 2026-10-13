import { test, expect } from "@playwright/test";

test("temporary demo workflows stay local and unmeasured", async ({
  page,
  browser,
}) => {
  const requests: string[] = [];
  const faults: string[] = [];
  page.on("request", (r) => {
    if (
      r.url().includes("/api/") ||
      !r.url().startsWith("http://localhost:3100/")
    )
      requests.push(r.url());
  });
  page.on("pageerror", (e) => faults.push(e.message));
  await page.goto("/");
  await expect(page.getByLabel("Demo mode")).toBeVisible();
  await expect(page.getByText("Your first endpoint starts here")).toBeVisible();
  await expect(page.getByLabel("Sign out")).toHaveCount(0);
  await page.getByRole("button", { name: "New monitor", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Display name").fill("Demo endpoint");
  await dialog.locator("[name=url]").fill("https://example.com");
  await dialog.locator("[name=authorized]").check();
  await dialog.locator("[name=public]").check();
  await expect(dialog.locator("[name=headers]")).toBeDisabled();
  await dialog.getByRole("button", { name: "Save monitor" }).click();
  await expect(dialog).not.toBeVisible();
  await expect(page.locator(".monitor-table")).toContainText("unknown");
  await expect(page.locator(".monitor-table")).toContainText("Awaiting data");
  await page.reload();
  await expect(page.getByLabel("Edit Demo endpoint")).toBeVisible();
  await page.getByLabel("Pause Demo endpoint").click();
  await expect(page.getByLabel("Resume Demo endpoint")).toBeVisible();
  await page.getByLabel("Resume Demo endpoint").click();
  await page.getByLabel("Edit Demo endpoint").click();
  await dialog.getByLabel("Display name").fill("Edited endpoint");
  await dialog.getByRole("button", { name: "Save monitor" }).click();
  await page.locator(".service-link").click();
  await expect(
    page.getByText("Live checks are disabled", { exact: false }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Settings & access", exact: true })
    .click();
  await expect(page.locator('.error[role="alert"]')).toHaveCount(0);
  await page.getByLabel("Page title").fill("My demo status");
  await page.getByRole("button", { name: "Save settings" }).click();
  await expect(page.getByText("Settings saved.")).toBeVisible();
  await expect(page.getByLabel("Current password")).toHaveCount(0);
  await page.getByRole("link", { name: "your status page" }).click();
  await expect(
    page.getByRole("heading", { name: "My demo status" }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Edited endpoint" }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Awaiting current service data" }),
  ).toBeVisible();
  // Another visitor has an independent, empty workspace.
  const visitor = await browser.newContext();
  const other = await visitor.newPage();
  await other.goto("http://localhost:3100/");
  await expect(
    other.getByText("Your first endpoint starts here"),
  ).toBeVisible();
  await visitor.close();
  await page.getByRole("link", { name: "Workspace", exact: true }).click();
  await page.getByRole("button", { name: "Channels", exact: true }).click();
  await expect(
    page.getByText("Alert delivery requires", { exact: false }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Monitors", exact: true }).click();
  await page.setViewportSize({ width: 390, height: 844 });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBeTruthy();
  await page.getByRole("button", { name: "Toggle colour theme" }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  page.once("dialog", (d) => d.accept());
  await page.getByLabel("Delete Edited endpoint").click();
  await expect(page.getByText("Your first endpoint starts here")).toBeVisible();
  expect(requests).toEqual([]);
  expect(faults).toEqual([]);
  // Direct requests cannot forward even with API_ORIGIN configured.
  for (const method of ["GET", "POST", "PUT", "DELETE"]) {
    const response = await page.request.fetch("/api/v1/monitors", {
      method,
      data: { name: "never forwarded" },
    });
    expect(response.status()).toBe(403);
    expect(await response.text()).toContain("disabled");
  }
});

test("expired and corrupt storage reset; manual reset clears local data", async ({
  page,
}) => {
  await page.goto("/");
  await expect(page.getByLabel("Demo mode")).toBeVisible();
  await page.evaluate(() => {
    const value = JSON.parse(sessionStorage.getItem("uptivra.demo.v1")!);
    value.expires = Date.now() - 1;
    value.settings.title = "Expired title";
    sessionStorage.setItem("uptivra.demo.v1", JSON.stringify(value));
  });
  await page.goto("/status");
  await expect(
    page.getByRole("heading", { name: "Demo service status" }),
  ).toBeVisible();
  await page.evaluate(() =>
    sessionStorage.setItem("uptivra.demo.v1", "invalid JSON"),
  );
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Demo service status" }),
  ).toBeVisible();
  page.once("dialog", (d) => d.accept());
  await page.getByRole("button", { name: "Reset demo" }).click();
  await expect(page.getByText("Your first endpoint starts here")).toBeVisible();
});

test("unavailable browser storage shows an error without collecting credentials", async ({
  page,
}) => {
  await page.addInitScript(() => {
    Storage.prototype.getItem = () => {
      throw new Error("Storage blocked");
    };
    Storage.prototype.setItem = () => {
      throw new Error("Storage blocked");
    };
    Storage.prototype.removeItem = () => {
      throw new Error("Storage blocked");
    };
  });
  await page.goto("/");
  await expect(page.locator('.error[role="alert"]')).toContainText(
    "Demo storage is unavailable",
  );
  await expect(page.locator("input[type=password]")).toHaveCount(0);
});
