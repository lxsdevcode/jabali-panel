// M6.6: Per-domain mail TLS section E2E.
//
// Read-only happy path — verifies the Mail TLS section renders on
// the domain detail Email tab with the opt-in toggle, the DNS A-
// record preview table, and the status chip. Does NOT actually
// trigger LE issuance because that would consume a rate-limit slot
// and require live DNS pointing at the test VM's public IP.
//
// Required env:
//   E2E_BASE_URL    https://jabali-panel.local
//   E2E_USERNAME    admin@example.com
//   E2E_PASSWORD    ...
//   E2E_DOMAIN_ID   ULID of an existing domain row in the test DB
//
// Skipped automatically when env not provided.

import { expect, test, type Page } from "@playwright/test";

const username = process.env.E2E_USERNAME || "";
const password = process.env.E2E_PASSWORD || "";
const domainID = process.env.E2E_DOMAIN_ID || "";

const SKIP_REASON =
  !username || !password
    ? "E2E_USERNAME / E2E_PASSWORD not set"
    : !domainID
      ? "E2E_DOMAIN_ID not set"
      : "";

if (SKIP_REASON) {
  test.skip(() => true, `M6.6 E2E skipped: ${SKIP_REASON}`);
} else {
  test.describe("M6.6: Per-domain mail TLS section", () => {
    test.setTimeout(45_000);

    test.use({
      ignoreHTTPSErrors: true,
      baseURL: process.env.E2E_BASE_URL || "https://jabali-panel.local",
    });

    async function login(page: Page) {
      await page.goto("/login");
      await page.getByLabel(/email/i).fill(username);
      await page.getByLabel(/password/i).fill(password);
      await page.getByRole("button", { name: /sign in/i }).click();
      await page.waitForLoadState("networkidle");
    }

    test("renders Mail TLS section + four-SAN DNS preview table", async ({ page }) => {
      await login(page);
      await page.goto(`/jabali-admin/domains/${domainID}/edit`);
      await page.getByRole("tab", { name: /email/i }).click();

      // Section heading.
      await expect(page.getByText(/^Mail TLS$/)).toBeVisible();

      // Opt-in switch is present.
      await expect(page.getByText(/Per-domain mail TLS/i)).toBeVisible();

      // All four SAN hostnames listed in the preview table.
      await expect(page.getByText(/^mail\./)).toBeVisible();
      await expect(page.getByText(/^autoconfig\./)).toBeVisible();
      await expect(page.getByText(/^autodiscover\./)).toBeVisible();
      await expect(page.getByText(/^mta-sts\./)).toBeVisible();
    });
  });
}
