// ModulesCard.test.tsx — M353 module install-on-enable status rendering.
// The card must reflect the real install state per module (not just the flag):
// an enabled module that isn't installed+active shows "not installed" + Retry,
// while an installed+active one shows "active". This is the whole point of Step
// 4 — surfacing that a flag-on module can still be down, and offering recovery.
import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("../../../apiClient", () => ({
  apiClient: { get: vi.fn(), patch: vi.fn(), post: vi.fn() },
}));

import { apiClient } from "../../../apiClient";
import { ModulesCard } from "./ModulesCard";

const mockGet = apiClient.get as ReturnType<typeof vi.fn>;

describe("ModulesCard install status", () => {
  beforeEach(() => vi.clearAllMocks());

  it("shows 'active' for an installed+active module and 'not installed' + Retry for an enabled-but-down one", async () => {
    mockGet.mockImplementation((url: string) => {
      if (url === "/admin/settings") {
        return Promise.resolve({
          data: { dns_enabled: true, mail_enabled: true, security_enabled: false, quota_enabled: true, api_enabled: true },
        });
      }
      if (url === "/admin/settings/modules/status") {
        return Promise.resolve({
          data: {
            modules: {
              dns: { installed: true, active: true },
              mail: { installed: false, active: false }, // enabled but down
              quota: { installed: true, active: true },
            },
          },
        });
      }
      return Promise.resolve({ data: {} });
    });

    render(<ModulesCard />);

    // dns is installed+active -> "active"; mail is enabled-but-down -> Retry.
    await waitFor(() => expect(screen.getAllByText("active").length).toBeGreaterThan(0));
    expect(screen.getByText("not installed")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /retry/i })).toBeInTheDocument();
  });
});
