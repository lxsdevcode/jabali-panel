import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { RouteErrorBoundary } from "./RouteErrorBoundary";
import { isChunkLoadError } from "./chunkError";

const RELOAD_FLAG = "jabali.chunkReloadAttempted";

const Boom = ({ error }: { error: Error }) => {
  throw error;
};

const chunkError = () =>
  new TypeError("Failed to fetch dynamically imported module: https://host/assets/Dashboard-a1b2.js");

describe("isChunkLoadError", () => {
  it("recognises the shapes browsers actually produce", () => {
    // Each engine words this differently and there is no typed error to match
    // on, so all three have to be recognised or the boundary silently stops
    // auto-recovering on that browser.
    for (const msg of [
      "Failed to fetch dynamically imported module: /assets/x-1.js", // Chrome
      "error loading dynamically imported module",                    // Firefox
      "Importing a module script failed.",                            // Safari
    ]) {
      expect(isChunkLoadError(new TypeError(msg)), msg).toBe(true);
    }
  });

  it("does not swallow ordinary application errors", () => {
    // A false positive here would reload the page on a real bug, hiding it and
    // looking like a random refresh to the user.
    expect(isChunkLoadError(new Error("Cannot read properties of undefined"))).toBe(false);
    expect(isChunkLoadError(new Error("Request failed with status code 500"))).toBe(false);
    expect(isChunkLoadError(null)).toBe(false);
  });
});

describe("RouteErrorBoundary", () => {
  beforeEach(() => {
    sessionStorage.clear();
    vi.spyOn(console, "error").mockImplementation(() => {});
  });
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("renders children when nothing throws", () => {
    render(
      <RouteErrorBoundary>
        <div>content</div>
      </RouteErrorBoundary>,
    );
    expect(screen.getByText("content")).toBeTruthy();
  });

  it("reloads once when a chunk fails to load", () => {
    // This is the deploy case: `jabali update` replaced the hashed chunks and
    // this tab is still holding the old index.html.
    const reload = vi.fn();
    render(
      <RouteErrorBoundary reload={reload}>
        <Boom error={chunkError()} />
      </RouteErrorBoundary>,
    );
    expect(reload).toHaveBeenCalledTimes(1);
    expect(sessionStorage.getItem(RELOAD_FLAG)).toBe("1");
  });

  it("does not reload a second time — a broken deploy must not loop", () => {
    // Without this guard a chunk that is genuinely gone would refresh the tab
    // forever, which is worse than the blank page it replaced.
    sessionStorage.setItem(RELOAD_FLAG, "1");
    const reload = vi.fn();
    render(
      <RouteErrorBoundary reload={reload}>
        <Boom error={chunkError()} />
      </RouteErrorBoundary>,
    );
    expect(reload).not.toHaveBeenCalled();
    expect(screen.getByText(/needs to be reloaded/i)).toBeTruthy();
  });

  it("shows a real error instead of a blank page for non-chunk failures", () => {
    const reload = vi.fn();
    render(
      <RouteErrorBoundary reload={reload}>
        <Boom error={new Error("boom")} />
      </RouteErrorBoundary>,
    );
    // Never auto-reload an application error — that would hide the bug and
    // present as a page that randomly refreshes itself.
    expect(reload).not.toHaveBeenCalled();
    expect(screen.getByText(/Something went wrong/i)).toBeTruthy();
  });

  it("clears the guard when the user retries by hand", () => {
    sessionStorage.setItem(RELOAD_FLAG, "1");
    const reload = vi.fn();
    render(
      <RouteErrorBoundary reload={reload}>
        <Boom error={chunkError()} />
      </RouteErrorBoundary>,
    );
    screen.getByRole("button", { name: /reload/i }).click();
    expect(reload).toHaveBeenCalledTimes(1);
    // Cleared so a later, unrelated stale chunk can still auto-recover.
    expect(sessionStorage.getItem(RELOAD_FLAG)).toBeNull();
  });
});
