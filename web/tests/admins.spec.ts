import { test, expect } from "@playwright/test";

test("owner creates an administrator who can sign in and is revoked on deletion", async ({
  page,
  browser,
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
  await page
    .getByRole("button", { name: "Settings & access", exact: true })
    .click();
  const email = `admin-${Date.now()}@example.com`;
  await page.getByLabel("New administrator email").fill(email);
  await page
    .getByLabel("Temporary password")
    .fill("browser-second-password-only");
  await page
    .getByRole("button", { name: "Create administrator", exact: true })
    .click();
  await expect(
    page.getByText("Administrator created.", { exact: true }),
  ).toBeVisible();
  const admins = await (await page.request.get("/api/v1/admins")).json();
  const admin = admins.find((a: { email: string }) => a.email === email);
  const context = await browser.newContext();
  const second = await context.newPage();
  try {
    await second.goto("/");
    await second.getByLabel("Email address").fill(email);
    await second
      .getByLabel("Password", { exact: true })
      .fill("browser-second-password-only");
    await second.getByRole("button", { name: "Sign in", exact: true }).click();
    await second
      .getByRole("button", { name: "Settings & access", exact: true })
      .click();
    await expect(
      second.getByRole("heading", { name: "Administrators & ownership" }),
    ).toBeVisible();
    await expect(
      second.getByRole("button", { name: "Create administrator", exact: true }),
    ).toHaveCount(0);
    const owner = admins.find((a: { owner: boolean }) => a.owner);
    expect(
      (
        await second.request.delete(`/api/v1/admins/${owner.id}`, { headers })
      ).status(),
    ).toBe(403);
    page.once("dialog", (dialog) => dialog.accept());
    await page
      .getByRole("button", {
        name: `Delete administrator ${email}`,
        exact: true,
      })
      .click();
    await expect(
      page.getByText("Administrator deleted.", { exact: true }),
    ).toBeVisible();
    expect((await second.request.get("/api/v1/auth/session")).status()).toBe(
      401,
    );
  } finally {
    await page.request.delete(`/api/v1/admins/${admin.id}`, { headers });
    await context.close();
  }
});
