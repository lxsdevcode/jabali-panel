// ServicesSummaryCard — an inactive on-demand/disabled unit (e.g. jabali-webmail,
// which the reconciler starts only once a domain enables email) must render as a
// neutral "idle", not a red "inactive" that reads like a failure. A unit that IS
// configured to run but is inactive stays red.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("../../../apiClient", () => ({ apiClient: { post: vi.fn() } }));
vi.mock("../../../hooks/useServerCapabilities", () => ({
  useServerCapabilities: () => ({ data: { postgres_enabled: true, docker_marketplace_enabled: true } }),
}));

import { ServicesSummaryCard } from "./ServicesSummaryCard";
import type { ServiceDetail } from "../../../hooks/useServerStatus";

const svc = (o: Partial<ServiceDetail>): ServiceDetail =>
  ({ unit: "x.service", active: "active", load_state: "loaded", sub_state: "", unit_file_state: "enabled", ...o }) as ServiceDetail;

function renderCard(services: ServiceDetail[]) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <ServicesSummaryCard services={services} />
    </QueryClientProvider>,
  );
}

describe("ServicesSummaryCard status", () => {
  it("shows on-demand inactive (disabled) as idle, not inactive", () => {
    renderCard([svc({ unit: "jabali-webmail.service", active: "inactive", unit_file_state: "disabled" })]);
    expect(screen.getByText("idle")).toBeInTheDocument();
    expect(screen.queryByText("inactive")).toBeNull();
  });

  it("shows a run-configured but inactive unit as inactive (red)", () => {
    renderCard([svc({ unit: "nginx.service", active: "inactive", unit_file_state: "enabled" })]);
    expect(screen.getByText("inactive")).toBeInTheDocument();
    expect(screen.queryByText("idle")).toBeNull();
  });

  it("shows an active unit as active", () => {
    renderCard([svc({ unit: "stalwart.service", active: "active", unit_file_state: "enabled" })]);
    expect(screen.getByText("active")).toBeInTheDocument();
  });
});
