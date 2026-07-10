import { describe, expect, it } from "vitest";
import { adminNav } from "./nav";

describe("adminNav", () => {
  it("puts Support last", () => {
    expect(adminNav[adminNav.length - 1].key).toBe("support");
  });

  it("gives every sidebar item a hover description", () => {
    const missing = adminNav.filter((n) => !n.description || n.description.trim() === "");
    expect(missing.map((n) => n.key)).toEqual([]);
  });
});
