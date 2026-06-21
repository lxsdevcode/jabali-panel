// CapabilityRoute — route guard for opt-in features (gap-audit #1). Wraps a
// page element; while the capability check is in flight it shows a spinner,
// renders the page when the flag is on, and redirects to the shell dashboard
// when off. Without this, a direct deep-link to e.g. /jabali-panel/python-apps
// rendered the page and immediately hit a backend 403 (broken-looking screen)
// even though the sidebar correctly hid the entry.
import type { ReactNode } from "react";

import { Spin } from "antd";
import { Navigate } from "react-router";

import { useServerCapabilities, type ServerCapabilities } from "../hooks/useServerCapabilities";

interface CapabilityRouteProps {
  cap: keyof ServerCapabilities;
  fallback: string;
  children: ReactNode;
}

export function CapabilityRoute({ cap, fallback, children }: CapabilityRouteProps) {
  const { data, isLoading } = useServerCapabilities();
  if (isLoading) {
    return (
      <div style={{ padding: 48, textAlign: "center" }}>
        <Spin />
      </div>
    );
  }
  if (!data?.[cap]) {
    return <Navigate to={fallback} replace />;
  }
  return <>{children}</>;
}
