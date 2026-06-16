// PythonAppsCard — opt-in toggle for the Python Application Manager
// (ADR-0131 / GH #203). Mirrors DockerMarketplaceCard. Flipping it on
// persists python_apps_enabled; the host installs the Python runtime
// prerequisites (python3-venv + build toolchain) via install.sh on the
// next `jabali update` (or already, on a fresh install with the flag set).
import { App, Alert, Card, Space, Switch, Tag, Typography } from "antd";
import { useEffect, useState } from "react";

import { apiClient } from "../../../apiClient";

interface ServerSettingsPyApps {
  python_apps_enabled: boolean;
}

export function PythonAppsCard() {
  const { message } = App.useApp();
  const [enabled, setEnabled] = useState<boolean | null>(null);
  const [busy, setBusy] = useState(false);

  const refresh = async () => {
    try {
      const r = await apiClient.get<ServerSettingsPyApps>("/admin/settings");
      setEnabled(!!r.data.python_apps_enabled);
    } catch (err) {
      message.error(
        `Could not load Python Apps setting: ${
          err instanceof Error ? err.message : String(err)
        }`,
      );
    }
  };

  useEffect(() => {
    void refresh();
  }, []);

  const toggle = async (value: boolean) => {
    setBusy(true);
    try {
      await apiClient.patch("/admin/settings", { python_apps_enabled: value });
      setEnabled(value);
      message.success(
        value
          ? "Python Application Manager enabled"
          : "Python Application Manager disabled",
      );
    } catch (err) {
      message.error(
        `Failed to update: ${err instanceof Error ? err.message : String(err)}`,
      );
    } finally {
      setBusy(false);
    }
  };

  return (
    <Card
      title={
        <Space>
          <span>Python Application Manager</span>
          {enabled ? <Tag color="green">Enabled</Tag> : <Tag>Disabled</Tag>}
        </Space>
      }
    >
      <Space direction="vertical" style={{ width: "100%" }} size="middle">
        <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
          Let users run native Python web apps (Django/Flask/FastAPI) on their
          domains — each app runs as an isolated per-user service behind nginx,
          like cPanel&apos;s Application Manager. PHP hosting is unaffected.
        </Typography.Paragraph>
        <Space>
          <Switch
            checked={!!enabled}
            loading={busy || enabled === null}
            onChange={(v) => void toggle(v)}
          />
          <Typography.Text>Enable Python apps</Typography.Text>
        </Space>
        {enabled && (
          <Alert
            type="info"
            showIcon
            message="Runtime prerequisites"
            description="Apps need python3-venv and a build toolchain (gcc, python3-dev, libffi/ssl-dev). These are installed by the host provisioner; if a venv build fails for a C-extension package, run jabali update to ensure the toolchain is present."
          />
        )}
      </Space>
    </Card>
  );
}
