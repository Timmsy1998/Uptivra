import { test, expect } from "@playwright/test";

test("Discord destinations can select and change a monitor group", async ({
  page,
}) => {
  const origin = process.env.E2E_ORIGIN || "http://localhost:3000";
  const headers = { Origin: origin };
  await page.goto("/");
  await page
    .getByLabel("Email address")
    .fill(process.env.ADMIN_EMAIL || "browser@example.com");
  await page
    .getByLabel("Password", { exact: true })
    .fill(process.env.ADMIN_PASSWORD || "browser-test-password-only");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "New monitor", exact: true }),
  ).toBeVisible();
  const created = await page.request.post("/api/v1/monitors", {
    headers,
    data: {
      name: "Bot group fixture",
      group: "Discord bots",
      type: "heartbeat",
      interval_seconds: 60,
      timeout_seconds: 5,
      expected_status: 200,
      failure_threshold: 1,
      recovery_threshold: 1,
      enabled: false,
      authorized: true,
      headers: {},
    },
  });
  expect(created.status()).toBe(201);
  const monitorID = (await created.json()).id;
  let channelID = "";
  try {
    await page.getByRole("button", { name: "Channels", exact: true }).click();
    await page
      .getByLabel("Name", { exact: true })
      .fill("Discord bots destination");
    await page.getByLabel("Provider").selectOption("discord");
    await page
      .getByLabel("Destination URL")
      .fill("https://discord.com/api/webhooks/test/test");
    await page
      .getByLabel("Monitor group")
      .selectOption(JSON.stringify("Discord bots"));
    await page.getByRole("button", { name: "Create channel" }).click();
    await expect(page.getByRole("status")).toContainText("Channel created.");
    const channels = await (await page.request.get("/api/v1/channels")).json();
    const channel = channels.find(
      (c: { name: string }) => c.name === "Discord bots destination",
    );
    channelID = channel.id;
    expect(channel.group).toBe("Discord bots");
    expect(channel.url).toBeUndefined();
    const selector = page.getByRole("combobox", {
      name: "Group for Discord bots destination",
      exact: true,
    });
    await expect(selector).toHaveValue(JSON.stringify("Discord bots"));
    for (const group of [null, "", "Discord bots"]) {
      await selector.selectOption(JSON.stringify(group));
      await expect(page.getByRole("status")).toContainText(
        "Group updated for future incident events.",
      );
      await expect(selector).toBeEnabled();
      const saved = await (await page.request.get("/api/v1/channels")).json();
      expect(saved.find((c: { id: string }) => c.id === channelID).group).toBe(
        group,
      );
    }
    // Keep an assigned group visible even when it no longer has monitors.
    expect(
      (
        await page.request.delete("/api/v1/monitors/" + monitorID, { headers })
      ).status(),
    ).toBe(204);
    await page.reload();
    await page.getByRole("button", { name: "Channels", exact: true }).click();
    await expect(selector).toHaveValue(JSON.stringify("Discord bots"));
    await page.setViewportSize({ width: 375, height: 812 });
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    ).toBe(true);
  } finally {
    if (channelID)
      await page.request.delete("/api/v1/channels/" + channelID, { headers });
    await page.request.delete("/api/v1/monitors/" + monitorID, { headers });
  }
});
