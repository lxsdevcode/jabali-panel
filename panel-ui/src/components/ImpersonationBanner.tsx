// ImpersonationBanner.tsx — persistent "Viewing as <user> — Exit" strip shown
// at the top of the user shell while an admin is acting-as (GH #183, ADR-0128).
// Read once per render; enter/exit do a full page navigation, so the banner is
// always correct on load. Sticky + a bright amber so it stays visible and
// clearly distinguishable in dark mode while scrolling.
import { Alert, Button } from "antd";

import { getActAs, stopImpersonation } from "../impersonation";

export function ImpersonationBanner() {
  const actAs = getActAs();
  if (!actAs) return null;

  const exit = async () => {
    await stopImpersonation();
    window.location.assign("/jabali-admin/users");
  };

  return (
    <div style={{ position: "sticky", top: 0, zIndex: 1001 }}>
      <Alert
        type="warning"
        banner
        showIcon
        // Force a bright amber in both themes — the default dark-mode warning
        // banner is near-black and doesn't read as an alert.
        style={{ background: "#fa8c16", borderBottom: "1px solid #ffc53d" }}
        message={
          <span style={{ color: "#1f1300", fontWeight: 600 }}>
            {`Viewing as ${actAs.username || actAs.id} — acting on this user's behalf. Every action is logged.`}
          </span>
        }
        action={
          <Button size="small" danger onClick={exit}>
            Exit
          </Button>
        }
      />
    </div>
  );
}
