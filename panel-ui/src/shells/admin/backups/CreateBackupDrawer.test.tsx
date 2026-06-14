// CreateBackupDrawer.test.tsx — regression for GH #182.
//
// The manual "Create backup" drawer showed an EMPTY user dropdown while
// the scheduled-backup form (SchedulesTab) showed users fine. Root
// cause: the drawer queried `admin/users`, but the user LIST route is
// `GET /api/v1/users` (`/admin/users` only has `:id/...` action
// sub-routes) — so the list 404'd and the Select rendered no options.
//
// These tests lock the wire contract: the drawer MUST hit `/users`
// (never `/admin/users`) and MUST render the non-admin accounts the
// list returns. Only apiClient is mocked; TanStack + AntD run for real.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { CreateBackupDrawer } from "./CreateBackupDrawer";

vi.mock("../../../apiClient", () => ({
  apiClient: { get: vi.fn(), post: vi.fn(), patch: vi.fn(), delete: vi.fn() },
}));

import { apiClient } from "../../../apiClient";

const mocked = apiClient as unknown as {
  get: ReturnType<typeof vi.fn>;
  post: ReturnType<typeof vi.fn>;
};

// The drawer issues two list GETs on open: users + backup-destinations.
// Route on the URL substring; envelope is the standard {data,total}.
function mockLists() {
  mocked.get.mockImplementation(async (url: string) => {
    if (url.startsWith("/users")) {
      return {
        data: {
          data: [
            { id: "u1", username: "alice", email: "alice@example.com", is_admin: false },
            { id: "u2", username: "root", email: "root@example.com", is_admin: true },
          ],
          total: 2,
        },
      };
    }
    if (url.includes("/backup-destinations")) {
      return { data: { data: [], total: 0 } };
    }
    throw new Error(`unexpected GET ${url}`);
  });
}

function renderDrawer() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={qc}>
      <CreateBackupDrawer open onClose={() => {}} onCreated={() => {}} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  mocked.get.mockReset();
  mocked.post.mockReset();
});

describe("CreateBackupDrawer — user list wire contract (GH #182)", () => {
  it("queries the /users list route, never /admin/users", async () => {
    mockLists();
    renderDrawer();

    await waitFor(() => {
      const urls = mocked.get.mock.calls.map((c) => String(c[0]));
      expect(urls.some((u) => /^\/users\?/.test(u))).toBe(true);
      expect(urls.some((u) => u.includes("/admin/users"))).toBe(false);
    });
  });

  it("renders non-admin accounts as selectable options and excludes admins", async () => {
    mockLists();
    renderDrawer();

    // Open the User select; AntD renders options in a body portal.
    const userSelect = await screen.findByText("Pick a user");
    fireEvent.mouseDown(userSelect);

    // alice (non-admin) is selectable; root (is_admin) is filtered out.
    expect(await screen.findByText(/alice \(alice@example\.com\)/)).toBeInTheDocument();
    expect(screen.queryByText(/root \(root@example\.com\)/)).toBeNull();
  });
});
