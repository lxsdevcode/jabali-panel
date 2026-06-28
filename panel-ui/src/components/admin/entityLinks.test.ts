import { describe, it, expect } from "vitest";

import {
  adminLinks,
  ownerLabel,
  ownerCrumbs,
  ownerResourceCrumbs,
  resourceListCrumbs,
  userResourceSiblings,
} from "./entityLinks";

describe("adminLinks", () => {
  it("builds plain list links", () => {
    expect(adminLinks.users()).toBe("/jabali-admin/users");
    expect(adminLinks.domains()).toBe("/jabali-admin/domains");
    expect(adminLinks.mailboxes()).toBe("/jabali-admin/mail");
    expect(adminLinks.dockerApps()).toBe("/jabali-admin/docker-apps");
    expect(adminLinks.backups()).toBe("/jabali-admin/backups");
  });

  it("appends ?user_id when owner-scoped", () => {
    expect(adminLinks.domains("01ARZ3NDEKTSV4RRFFQ69G5FAV")).toBe(
      "/jabali-admin/domains?user_id=01ARZ3NDEKTSV4RRFFQ69G5FAV",
    );
    expect(adminLinks.mailboxes("u1")).toBe("/jabali-admin/mail?user_id=u1");
  });

  it("builds detail links", () => {
    expect(adminLinks.user("u1")).toBe("/jabali-admin/users/u1");
    expect(adminLinks.domainEdit("d1")).toBe("/jabali-admin/domains/edit/d1");
    expect(adminLinks.packageEdit("p1")).toBe("/jabali-admin/packages/edit/p1");
  });

  it("url-encodes ids", () => {
    expect(adminLinks.user("a/b")).toBe("/jabali-admin/users/a%2Fb");
  });
});

describe("ownerLabel", () => {
  it("prefers username", () => {
    expect(ownerLabel({ id: "01ARZ3NDEKTSV4RRFFQ69G5FAV", username: "alice" })).toBe("alice");
  });
  it("falls back to short id when username missing or blank", () => {
    expect(ownerLabel({ id: "01ARZ3NDEKTSV4RRFFQ69G5FAV", username: null })).toBe("01ARZ3ND");
    expect(ownerLabel({ id: "01ARZ3NDEKTSV4RRFFQ69G5FAV", username: "  " })).toBe("01ARZ3ND");
  });
});

describe("userResourceSiblings", () => {
  it("returns the other owner-scoped resources, excluding the current one", () => {
    const sibs = userResourceSiblings("u1", "domains");
    expect(sibs.map((s) => s.key)).toEqual(["mailboxes", "docker-apps", "backups"]);
    expect(sibs.every((s) => s.href.includes("user_id=u1"))).toBe(true);
  });
  it("returns all four when no current key", () => {
    expect(userResourceSiblings("u1")).toHaveLength(4);
  });
});

describe("crumb builders", () => {
  const user = { id: "u1", username: "alice" };

  it("ownerCrumbs links Users, leaves the name as the leaf", () => {
    const c = ownerCrumbs(user);
    expect(c).toEqual([
      { title: "Users", href: "/jabali-admin/users" },
      { title: "alice" },
    ]);
  });

  it("ownerResourceCrumbs links Users + name and adds a sibling menu on the resource", () => {
    const c = ownerResourceCrumbs(user, { key: "domains", label: "Domains" });
    expect(c[0]).toEqual({ title: "Users", href: "/jabali-admin/users" });
    expect(c[1]).toEqual({ title: "alice", href: "/jabali-admin/users/u1" });
    expect(c[2].title).toBe("Domains");
    expect(c[2].href).toBeUndefined();
    expect(c[2].menu?.map((m) => m.key)).toEqual(["mailboxes", "docker-apps", "backups"]);
  });

  it("resourceListCrumbs is a single leaf", () => {
    expect(resourceListCrumbs("Domains")).toEqual([{ title: "Domains" }]);
  });
});
