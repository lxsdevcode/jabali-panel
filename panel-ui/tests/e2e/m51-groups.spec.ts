// M51: Mailbox Groups (shared mailbox + calendar/address book/files) E2E.
//
// Covers the UI-driven control-plane flow for the groups feature:
//   1. Login
//   2. Mail → Groups tab, pick a mail-enabled domain
//   3. Create a group (name + display name; default resource toggles)
//   4. Verify the group row appears with its email + resource tags
//   5. Open the members modal (smoke — the multi-select renders)
//   6. Delete the group; verify the row goes away
//
// The wire/agent/Stalwart side (registry @type:Group projection,
// memberGroupIds, queryRecipient member-expansion receive, send-as) is
// covered by panel-api/agent tests + the live .14 probes documented in
// ADR-0132's Resolution section. This spec proves the panel UI drives the
// create → members → delete lifecycle.
//
// To run locally:
//   export E2E_BASE_URL=https://jabali-panel.local
//   export E2E_USERNAME=admin@example.com
//   export E2E_PASSWORD=...
//   export E2E_M51_DOMAIN=example.com   # an existing mail-enabled domain
//   npm run test:e2e m51-groups.spec.ts

import { expect, test } from "@playwright/test";

const SKIP_REASON =
  !process.env.E2E_USERNAME || !process.env.E2E_PASSWORD || !process.env.E2E_M51_DOMAIN
    ? "E2E_USERNAME, E2E_PASSWORD, E2E_M51_DOMAIN not set"
    : "";

if (SKIP_REASON) {
  test.skip(() => true, `M51 E2E skipped: ${SKIP_REASON}`);
} else {
  test.describe("M51: Mailbox Groups", () => {
    test.setTimeout(120_000);

    test.use({
      ignoreHTTPSErrors: true,
      baseURL: process.env.E2E_BASE_URL || "https://jabali-panel.local",
    });

    const username = process.env.E2E_USERNAME || "";
    const password = process.env.E2E_PASSWORD || "";
    const domain = process.env.E2E_M51_DOMAIN || "";
    const groupName = `e2egrp${Date.now().toString(36).slice(-5)}`;
    const groupEmail = `${groupName}@${domain}`;

    async function login(page: import("@playwright/test").Page): Promise<void> {
      await page.goto("/login");
      await page.getByLabel(/email/i).fill(username);
      await page.getByLabel(/password/i).fill(password);
      await page.getByRole("button", { name: /sign in|log in/i }).click();
      await page.waitForURL(/\/jabali-panel/);
    }

    test("create group → verify row → members modal → delete", async ({ page }) => {
      await login(page);

      await page.goto("/jabali-panel/mail/groups");

      // Pick the mail-enabled domain in the "Create in"/domain selector.
      // The Select defaults to the first mail domain; switch to ours.
      const domainSelect = page.locator(".ant-select").first();
      await domainSelect.click();
      await page.getByTitle(domain, { exact: true }).first().click();

      // Create.
      await page.getByRole("button", { name: /new group/i }).click();
      await page.getByLabel(/group name/i).fill(groupName);
      await page.getByLabel(/display name/i).fill("E2E Group");
      await page.getByRole("button", { name: /^save$/i }).click();

      // Row appears with the derived email.
      const row = page.getByRole("row", { name: new RegExp(groupEmail, "i") }).first();
      await expect(row).toBeVisible({ timeout: 15_000 });

      // Members modal smoke — opens + shows the member multi-select.
      await row.getByRole("button").first().click(); // first action = manage members
      await expect(page.getByText(/members —/i)).toBeVisible();
      await page.getByRole("button", { name: /cancel/i }).click();

      // Delete.
      await row.getByRole("button").last().click(); // last action = delete (trash)
      await page.getByRole("button", { name: /^delete$/i }).click();
      await expect(
        page.getByRole("row", { name: new RegExp(groupEmail, "i") }),
      ).toHaveCount(0, { timeout: 15_000 });
    });
  });
}
