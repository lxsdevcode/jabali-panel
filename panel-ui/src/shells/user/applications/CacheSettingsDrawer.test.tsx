// CacheSettingsDrawer.test.tsx — GH #616. Locks the drawer's wire contract
// against the L1 envelope confirmed by applications_cache_settings_test.go, and
// runs the real AntD Drawer/Form/Select so a runtime React/AntD break surfaces
// here (tsc / npm build don't catch those). Only apiClient is mocked.
import { App } from "antd";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { CacheSettingsDrawer } from "./CacheSettingsDrawer";

vi.mock("../../../apiClient", () => ({
  apiClient: { get: vi.fn(), put: vi.fn() },
}));

import { apiClient } from "../../../apiClient";

const mocked = apiClient as unknown as {
  get: ReturnType<typeof vi.fn>;
  put: ReturnType<typeof vi.fn>;
};

function renderDrawer() {
  render(
    <App>
      <CacheSettingsDrawer
        install={{ id: "inst1", name: "example.com", cache_enabled: true }}
        onClose={() => {}}
      />
    </App>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe("CacheSettingsDrawer", () => {
  it("GETs the right route, loads existing url_exclusions, and saves them back", async () => {
    mocked.get.mockResolvedValue({
      data: {
        cache_enabled: true,
        configured: true,
        settings: { url_exclusions: ["/private"] },
      },
    });
    mocked.put.mockResolvedValue({ data: { ok: true } });
    renderDrawer();

    await waitFor(() =>
      expect(mocked.get).toHaveBeenCalledWith(
        "/applications/inst1/cache-settings",
      ),
    );
    // the loaded exclusion renders as a tag
    await screen.findByText("/private");

    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() =>
      expect(mocked.put).toHaveBeenCalledWith(
        "/applications/inst1/cache-settings",
        expect.objectContaining({ url_exclusions: ["/private"] }),
      ),
    );
  });

  it("renders unconfigured without crashing and saves an empty list (no fabricated fields)", async () => {
    mocked.get.mockResolvedValue({
      data: { cache_enabled: true, configured: false, settings: {} },
    });
    mocked.put.mockResolvedValue({ data: { ok: true } });
    renderDrawer();

    await waitFor(() => expect(mocked.get).toHaveBeenCalled());
    await screen.findByText(/Object cache/);

    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() =>
      expect(mocked.put).toHaveBeenCalledWith(
        "/applications/inst1/cache-settings",
        expect.objectContaining({ url_exclusions: [] }),
      ),
    );
  });
});
