import { useEffect, useState } from "react";
import { Card, Space, Switch, Typography, notification } from "antd";

import { apiClient } from "../../../apiClient";

// ModulesCard — Server Settings → Modules (M353 Phase 1, GH #353). Enable or
// disable optional panel modules server-wide. When a module is off the panel
// hides its nav + pages (serverCapabilities → CapabilityRoute) and the backend
// rejects its endpoints (409). Turning a module ON here sets the flag; the
// underlying software install/removal is handled at install time (JABALI_MODULES)
// and, later, a runtime provision step. Flags default ON so existing installs
// keep every feature.
type ModuleKey = "dns_enabled" | "mail_enabled" | "security_enabled" | "quota_enabled" | "api_enabled";

const MODULES: { key: ModuleKey; label: string; desc: string }[] = [
  { key: "dns_enabled", label: "DNS server (PowerDNS)", desc: "Authoritative DNS + the domain records / DNSSEC pages." },
  { key: "mail_enabled", label: "Mail server (Stalwart + Bulwark)", desc: "Mailboxes, forwarders, webmail, and the Mail pages." },
  { key: "security_enabled", label: "Security (CrowdSec, malware/ClamAV, AppArmor)", desc: "Intrusion detection, malware scanning, and the Security page." },
  { key: "quota_enabled", label: "Filesystem quota", desc: "Per-user disk quota enforcement + the quota fields." },
  { key: "api_enabled", label: "REST API (API keys)", desc: "Remote-management API keys + the API Tokens page." },
];

export const ModulesCard = () => {
  const [state, setState] = useState<Record<ModuleKey, boolean>>({
    dns_enabled: true,
    mail_enabled: true,
    security_enabled: true,
    quota_enabled: true,
    api_enabled: true,
  });
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState<ModuleKey | null>(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const resp = await apiClient.get<Partial<Record<ModuleKey, boolean>>>("/admin/settings");
        if (!cancelled) {
          setState((prev) => {
            const next = { ...prev };
            for (const m of MODULES) next[m.key] = resp.data[m.key] !== false;
            return next;
          });
        }
      } catch {
        if (!cancelled) notification.error({ message: "Failed to load module settings" });
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const onToggle = async (key: ModuleKey, label: string, next: boolean) => {
    setSaving(key);
    try {
      await apiClient.patch("/admin/settings", { [key]: next });
      setState((prev) => ({ ...prev, [key]: next }));
      notification.success({
        message: `${label} ${next ? "enabled" : "disabled"}`,
        description: next
          ? "The module's pages are now available."
          : "The module's pages are hidden; its endpoints return 409.",
      });
    } catch {
      notification.error({ message: `Failed to update ${label}` });
    } finally {
      setSaving(null);
    }
  };

  return (
    <Card title="Modules" style={{ marginBottom: 16 }} loading={loading}>
      <Typography.Paragraph type="secondary" style={{ marginTop: 0 }}>
        Turn optional modules on or off. A disabled module disappears from the
        panel (nav + pages) and its endpoints are rejected — existing data is not
        removed. Core services (web, database, panel) are always on.
      </Typography.Paragraph>
      <Space direction="vertical" size="middle" style={{ width: "100%" }}>
        {MODULES.map((m) => (
          <Space key={m.key} align="start" style={{ width: "100%", justifyContent: "space-between" }}>
            <div style={{ maxWidth: 520 }}>
              <Typography.Text strong>{m.label}</Typography.Text>
              <br />
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                {m.desc}
              </Typography.Text>
            </div>
            <Switch
              checked={state[m.key]}
              loading={saving === m.key}
              onChange={(next) => onToggle(m.key, m.label, next)}
              checkedChildren="On"
              unCheckedChildren="Off"
            />
          </Space>
        ))}
      </Space>
    </Card>
  );
};
