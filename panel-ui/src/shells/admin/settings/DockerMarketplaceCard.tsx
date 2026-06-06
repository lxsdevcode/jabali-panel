// DockerMarketplaceCard — toggle the M48 docker app marketplace
// opt-in. Mirrors DatabasesCard (postgres opt-in) shape.
//
// Flip true: panel-api dispatches `docker.install` on the agent
//   (sources install.sh, runs install_docker_engine, starts docker).
// Flip false: panel-api dispatches `docker.disable` (stops + disables
//   docker units). Data under /var/lib/jabali/docker-apps/ stays.
import { App, Alert, Card, Space, Spin, Switch, Tag, Typography } from "antd";
import { useEffect, useState } from "react";

import { apiClient } from "../../../apiClient";

interface ServerSettingsDocker {
  docker_marketplace_enabled: boolean;
}

export function DockerMarketplaceCard() {
  const { message } = App.useApp();
  const [enabled, setEnabled] = useState<boolean | null>(null);
  const [busy, setBusy] = useState(false);
  const [busyLabel, setBusyLabel] = useState<string | null>(null);

  const refresh = async () => {
    try {
      const r = await apiClient.get<ServerSettingsDocker>("/admin/settings");
      setEnabled(!!r.data.docker_marketplace_enabled);
    } catch (err) {
      message.error(
        `Could not load marketplace setting: ${
          err instanceof Error ? err.message : String(err)
        }`,
      );
    }
  };

  useEffect(() => {
    void refresh();
  }, []);

  const persistFlag = async (value: boolean) => {
    await apiClient.patch("/admin/settings", {
      docker_marketplace_enabled: value,
    });
    setEnabled(value);
  };

  const handleEnable = async () => {
    setBusy(true);
    setBusyLabel("Installing Docker — this may take a few minutes…");
    try {
      await persistFlag(true);
      // Backend dispatches `docker.install` as fire-and-forget so the
      // PATCH does not block. Poll docker.status every 4s until the
      // engine is installed AND the daemon is active, then drop the
      // spinner. Cap at 5 minutes; the docker.install agent verb has
      // its own 10-min ceiling.
      const deadline = Date.now() + 5 * 60 * 1000;
      while (Date.now() < deadline) {
        await new Promise((r) => setTimeout(r, 4000));
        try {
          const { data } = await apiClient.get<{ installed?: boolean; active?: boolean }>(
            "/admin/docker-apps/engine/status",
          );
          if (data?.installed && data?.active) {
            message.success("Docker engine installed + active. Marketplace is live.");
            return;
          }
        } catch {
          // transient -- agent may be unreachable mid-install. Keep polling.
        }
        setBusyLabel("Installing Docker — still in progress…");
      }
      message.warning("Install poll timed out; check Server Status -> Docker for live state.");
    } catch (err) {
      message.error(err instanceof Error ? err.message : "Failed to enable");
    } finally {
      setBusy(false);
      setBusyLabel(null);
    }
  };

  const handleDisable = async () => {
    setBusy(true);
    setBusyLabel("Disabling Docker…");
    try {
      await persistFlag(false);
      message.warning("Marketplace disabled. App data preserved under /var/lib/jabali/docker-apps/.");
    } catch (err) {
      message.error(err instanceof Error ? err.message : "Failed to disable");
    } finally {
      setBusy(false);
      setBusyLabel(null);
    }
  };

  if (enabled === null) {
    return (
      <Card title="Docker App Marketplace (M48)">
        <Spin />
      </Card>
    );
  }

  return (
    <Card
      title={
        <Space>
          Docker App Marketplace (M48)
          {enabled ? <Tag color="green">Enabled</Tag> : <Tag>Disabled</Tag>}
        </Space>
      }
    >
      <Space direction="vertical" size="middle" style={{ width: "100%" }}>
        <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
          Curated catalog of one-click Docker apps (Vaultwarden, Uptime Kuma, Gitea, …) with
          auto domain + nginx reverse proxy + LE TLS + restic backup. Admin-installable only.
          Disabled by default; enabling installs Docker CE (~400 MB) and adds a docker daemon
          to the host.
        </Typography.Paragraph>

        {!enabled && (
          <Alert
            type="info"
            showIcon
            message="Opt-in feature"
            description={
              <>
                Click <b>Enable</b> below to install Docker and unlock the marketplace at{" "}
                <code>/jabali-admin/docker-apps</code>. You can disable any time; app data
                under <code>/var/lib/jabali/docker-apps/</code> survives the flip.
              </>
            }
          />
        )}

        {enabled && (
          <Alert
            type="success"
            showIcon
            message="Marketplace live"
            description={
              <>
                Open <a href="/jabali-admin/docker-apps">/jabali-admin/docker-apps</a> to
                install apps from the catalog.
              </>
            }
          />
        )}

        <Space>
          <Switch
            checked={enabled}
            loading={busy}
            onChange={(v) => (v ? handleEnable() : handleDisable())}
          />
          <Typography.Text>
            {enabled ? "Disable marketplace" : "Enable marketplace"}
          </Typography.Text>
        </Space>
        {busyLabel && <Typography.Text type="secondary">{busyLabel}</Typography.Text>}
      </Space>
    </Card>
  );
}
