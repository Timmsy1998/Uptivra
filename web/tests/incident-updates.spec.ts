import { execFileSync } from "node:child_process";
import { test, expect } from "@playwright/test";

let monitorID: string | undefined;

test.afterEach(async ({ page }) => {
  if (!monitorID) return;
  const id = monitorID;
  monitorID = undefined;
  expect(
    (
      await page.request.delete(`/api/v1/monitors/${id}`, {
        headers: { Origin: process.env.E2E_ORIGIN || "http://localhost:3000" },
        timeout: 10000,
      })
    ).status(),
  ).toBe(204);
});

test("publish fix details and read them on the public incident timeline", async ({
  page,
  request,
}) => {
  expect(
    process.env.TEST_DATABASE_URL,
    "Set TEST_DATABASE_URL to the isolated browser-test database",
  ).toBeTruthy();
  const origin = process.env.E2E_ORIGIN || "http://localhost:3000";
  await page.goto("/");
  await page
    .getByLabel("Email address")
    .fill(process.env.ADMIN_EMAIL || "browser@example.com");
  await page
    .getByLabel("Password", { exact: true })
    .fill(process.env.ADMIN_PASSWORD || "browser-test-password-only");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Every endpoint. A clear picture." }),
  ).toBeVisible();
  const name = `Incident notes ${Date.now()}`;
  const response = await page.request.post("/api/v1/monitors", {
    headers: { Origin: origin },
    data: {
      name,
      type: "heartbeat",
      interval_seconds: 60,
      grace_seconds: 0,
      failure_threshold: 1,
      recovery_threshold: 1,
      enabled: true,
      public: true,
      authorized: true,
    },
  });
  expect(response.status()).toBe(201);
  const { id } = await response.json();
  monitorID = id;
  // Seed only the explicit local test database, avoiding a minute-long heartbeat wait.
  execFileSync(
    "psql",
    [
      process.env.TEST_DATABASE_URL!,
      "-v",
      "ON_ERROR_STOP=1",
      "-v",
      `monitor_id=${id}`,
    ],
    {
      input:
        "BEGIN; UPDATE monitors SET status='down',failures=1,successes=0 WHERE id=:'monitor_id'; INSERT INTO incidents(id,monitor_id) VALUES (:'monitor_id',:'monitor_id'); COMMIT;",
      stdio: ["pipe", "pipe", "pipe"],
    },
  );
  const incidents = await (await page.request.get("/api/v1/incidents")).json();
  const incident = incidents.find(
    (i: { monitor_id: string }) => i.monitor_id === id,
  );
  expect(
    incident,
    "Seed the incident fixture before running this browser workflow",
  ).toBeTruthy();
  await page.reload();
  await page.getByRole("button", { name: /^Incidents(?: \d+)?$/ }).click();
  const card = page
    .locator(".incident-list article")
    .filter({ has: page.getByRole("heading", { name, exact: true }) });
  await card.getByText("Add incident update", { exact: true }).click();
  await card.getByLabel("Update stage").selectOption("monitoring");
  const message =
    "Fix deployed. Watching recovery.\n<script>alert('literal text')</script>";
  await card.getByLabel("Incident details").fill(message);
  await card.getByRole("button", { name: "Publish update" }).click();
  await expect(
    card.getByText("Update published.", { exact: true }),
  ).toBeVisible();
  await expect(card.locator(".incident-message")).toHaveText(message);
  const publicPage = await page.context().newPage();
  await publicPage.goto("/status");
  const publicCard = publicPage
    .locator(".incident-list article")
    .filter({ has: publicPage.getByRole("heading", { name, exact: true }) });
  await expect(publicCard.locator(".incident-message")).toHaveText(message);
  await expect(publicCard.getByText("Add incident update")).toHaveCount(0);
  expect(
    (
      await request.post(`/api/v1/incidents/${incident.id}/updates`, {
        data: { status: "update", message: "Unauthorized" },
      })
    ).status(),
  ).toBe(401);
});
