// RouteErrorBoundary — catches route-chunk load failures and recovers.
//
// The app splits into 56 lazy() route chunks and had no error boundary at all.
// Suspense only handles a PENDING import; a REJECTED one throws past it, and
// with nothing to catch it React unmounts the tree. The user gets a blank
// screen — no message, no recovery, nothing in the UI to act on.
//
// This is not hypothetical, and it is not rare. Chunk filenames are content
// hashed, and webui.go deliberately 404s an unknown asset rather than serving
// index.html in its place:
//
//	(open across a deploy) must 404 — NOT fall through to index.html.
//
// That is the right server behaviour — handing back HTML for a .js request
// would fail more confusingly. But it means every `jabali update` strands any
// browser still holding the previous index.html: the moment that tab navigates
// to a lazy route, it requests a chunk name that no longer exists, gets a 404,
// and goes blank. Reloading fixes it because the fresh index.html references
// the new hashes — which is exactly the "it goes blank, a refresh fixes it"
// report this was written for.
//
// So a stale chunk gets ONE automatic reload, guarded by a sessionStorage flag
// so a genuinely broken deploy cannot put the tab in a refresh loop. Anything
// else — or a reload that did not help — renders an error with a manual retry
// instead of a blank page.
import { Component, type ErrorInfo, type ReactNode } from "react";
import { Button, Result } from "antd";
import { isChunkLoadError } from "./chunkError";

const RELOAD_FLAG = "jabali.chunkReloadAttempted";

interface Props {
  children: ReactNode;
  /** Injected in tests so the reload path can be asserted without navigating. */
  reload?: () => void;
}

interface State {
  error: Error | null;
  /** True when we already reloaded once and the chunk still would not load. */
  reloadDidNotHelp: boolean;
}

export class RouteErrorBoundary extends Component<Props, State> {
  state: State = { error: null, reloadDidNotHelp: false };

  static getDerivedStateFromError(error: Error): Partial<State> {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    // Keep the stack in the console — the UI below deliberately does not show
    // it, but whoever is debugging needs more than "something went wrong".
    console.error("[route] render failed", error, info.componentStack);

    if (!isChunkLoadError(error)) return;

    let alreadyTried = false;
    try {
      alreadyTried = sessionStorage.getItem(RELOAD_FLAG) === "1";
    } catch {
      // Private mode / storage disabled. Treat as "already tried" so we show
      // the error rather than risking a reload loop we cannot detect.
      alreadyTried = true;
    }

    if (alreadyTried) {
      this.setState({ reloadDidNotHelp: true });
      return;
    }

    try {
      sessionStorage.setItem(RELOAD_FLAG, "1");
    } catch {
      /* best effort */
    }
    (this.props.reload ?? (() => window.location.reload()))();
  }

  private handleRetry = () => {
    try {
      sessionStorage.removeItem(RELOAD_FLAG);
    } catch {
      /* best effort */
    }
    (this.props.reload ?? (() => window.location.reload()))();
  };

  render() {
    const { error, reloadDidNotHelp } = this.state;
    if (!error) return this.props.children;

    const chunk = isChunkLoadError(error);
    // A chunk error we have not yet reloaded for is about to reload the page;
    // render nothing rather than flashing an error the user cannot read.
    if (chunk && !reloadDidNotHelp) return null;

    return (
      <Result
        status={chunk ? "warning" : "error"}
        title={chunk ? "This page needs to be reloaded" : "Something went wrong"}
        subTitle={
          chunk
            ? "The panel was updated while this tab was open, so part of it could no longer be loaded. Reloading picks up the new version."
            : "An unexpected error stopped this page from rendering. Reloading usually clears it; if it keeps happening the details are in the browser console."
        }
        extra={
          <Button type="primary" onClick={this.handleRetry}>
            Reload
          </Button>
        }
      />
    );
  }
}

export default RouteErrorBoundary;
