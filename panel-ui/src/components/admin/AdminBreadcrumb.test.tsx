import { render } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { describe, it, expect } from "vitest";

import { AdminBreadcrumb } from "./AdminBreadcrumb";
import { ownerResourceCrumbs } from "./entityLinks";

const renderInRouter = (ui: React.ReactNode) =>
  render(<MemoryRouter>{ui}</MemoryRouter>);

describe("AdminBreadcrumb", () => {
  it("renders linked crumbs and a plain leaf", () => {
    const { getByText } = renderInRouter(
      <AdminBreadcrumb
        items={ownerResourceCrumbs({ id: "u1", username: "alice" }, { key: "domains", label: "Domains" })}
      />,
    );
    // Users + alice are links; Domains is the leaf.
    const users = getByText("Users").closest("a");
    expect(users?.getAttribute("href")).toBe("/jabali-admin/users");
    const alice = getByText("alice").closest("a");
    expect(alice?.getAttribute("href")).toBe("/jabali-admin/users/u1");
    // Domains crumb is now a link to its owner-scoped list.
    expect(getByText("Domains").closest("a")?.getAttribute("href")).toBe(
      "/jabali-admin/domains?user_id=u1",
    );
  });

  it("renders a sibling dropdown caret when a crumb has a menu", () => {
    const { container } = renderInRouter(
      <AdminBreadcrumb
        items={ownerResourceCrumbs({ id: "u1", username: "alice" }, { key: "domains", label: "Domains" })}
      />,
    );
    // AntD renders the dropdown trigger with this class when menu items exist.
    expect(container.querySelector(".ant-breadcrumb-overlay-link")).not.toBeNull();
  });
});
