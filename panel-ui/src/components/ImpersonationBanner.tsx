// ImpersonationBanner.tsx — persistent "Viewing as <user> — Exit" strip shown
// at the top of the user shell while an admin is acting-as (GH #183, ADR-0128).
// Read once per render; enter/exit do a full page navigation, so the banner is
// always correct on load.
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
    <Alert
      type="warning"
      banner
      showIcon
      message={`Viewing as ${actAs.email} — acting on this user's behalf. Every action is logged.`}
      action={
        <Button size="small" danger onClick={exit}>
          Exit
        </Button>
      }
    />
  );
}
