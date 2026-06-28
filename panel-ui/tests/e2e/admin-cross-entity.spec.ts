// Admin cross-entity navigation E2E (#483, ADR-0152) — drives the headline
// drill path with a mocked backend: Users -> user hub -> owner-scoped Domains
// -> DomainEdit -> Owner link. Asserts the breadcrumbs + owner chip that tie
// the admin entities together.
import { admin, expect, mockApi, signIn, test, type MockDomain, type MockUser } from "./fixtures";

const now = "2026-01-01T00:00:00Z";

const owner: MockUser = {
  id: "01BX5ZZKBKACTAV9WEVGEMMVRZ",
  email: "alice@example.com",
  username: "alice",
  name_first: "Alice",
  name_last: "Smith",
  is_admin: false,
  created_at: now,
  updated_at: now,
};

const domain: MockDomain = {
  id: "01CKBYK3W2VHYBR9V8FT9X3Z8T",
  user_id: owner.id,
  username: "alice",
  name: "example.com",
  doc_root: "/home/alice/domains/example.com/public_html",
  is_enabled: true,
  nginx_custom_directives: "",
  created_at: now,
  updated_at: now,
};

test.describe("admin cross-entity navigation (#483)", () => {
  test("Users -> hub -> owner-scoped Domains -> DomainEdit owner link", async ({ page }) => {
    await mockApi(page, { me: admin, users: [admin, owner], domains: [domain] });
    await signIn(page, admin);
    await page.waitForURL(/\/jabali-admin/);

    // 1) Users list: the owner's username links to the hub.
    await page.goto("/jabali-admin/users");
    await page.getByRole("link", { name: "alice" }).click();
    await page.waitForURL(new RegExp(`/jabali-admin/users/${owner.id}`));

    // 2) Hub: breadcrumb Users / alice + a Domains card linking owner-scoped.
    await expect(page.locator(".ant-breadcrumb")).toContainText("Users");
    await expect(page.locator(".ant-breadcrumb")).toContainText("alice");
    const domainsCard = page.getByRole("link", { name: /Domains/ });
    await expect(domainsCard.first()).toHaveAttribute(
      "href",
      `/jabali-admin/domains?user_id=${owner.id}`,
    );

    // 3) Click the Domains card -> owner-scoped DomainList.
    await domainsCard.first().click();
    await page.waitForURL(new RegExp(`/jabali-admin/domains\\?user_id=${owner.id}`));

    // 4) DomainList shows the owner breadcrumb + removable Owner chip.
    await expect(page.locator(".ant-breadcrumb")).toContainText("Domains");
    await expect(page.getByText("Owner: alice")).toBeVisible();

    // 5) DomainEdit: breadcrumb Users / alice / Domains / example.com + Owner link.
    await page.goto(`/jabali-admin/domains/edit/${domain.id}`);
    await expect(page.locator(".ant-breadcrumb")).toContainText("example.com");
    const ownerLink = page.getByRole("link", { name: "alice" }).first();
    await expect(ownerLink).toHaveAttribute("href", `/jabali-admin/users/${owner.id}`);
  });
});
