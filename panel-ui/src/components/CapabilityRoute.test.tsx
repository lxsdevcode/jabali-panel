// CapabilityRoute.test.tsx — gap-audit #1. The guard must redirect a deep-link
// to a disabled feature instead of rendering the page (which then 403s), render
// the page when the capability is on, and not flash either while the check is
// in flight.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { describe, expect, it, vi } from "vitest";

import { CapabilityRoute } from "./CapabilityRoute";

const getMock = vi.fn();
vi.mock("../apiClient", () => ({
  apiClient: { get: (...args: unknown[]) => getMock(...args) },
}));

function renderAt(caps: Record<string, boolean>) {
  getMock.mockResolvedValue({ data: caps });
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={["/feature"]}>
        <Routes>
          <Route
            path="/feature"
            element={
              <CapabilityRoute cap="python_apps_enabled" fallback="/dashboard">
                <div>FEATURE PAGE</div>
              </CapabilityRoute>
            }
          />
          <Route path="/dashboard" element={<div>DASHBOARD</div>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("CapabilityRoute", () => {
  it("renders the page when the capability is enabled", async () => {
    renderAt({ python_apps_enabled: true });
    expect(await screen.findByText("FEATURE PAGE")).toBeInTheDocument();
  });

  it("redirects to the fallback when the capability is disabled", async () => {
    renderAt({ python_apps_enabled: false });
    await waitFor(() => expect(screen.getByText("DASHBOARD")).toBeInTheDocument());
    expect(screen.queryByText("FEATURE PAGE")).not.toBeInTheDocument();
  });

  it("does not render the page before the check resolves", () => {
    renderAt({ python_apps_enabled: true });
    // synchronous first paint: still loading, page not yet shown
    expect(screen.queryByText("FEATURE PAGE")).not.toBeInTheDocument();
  });
});
