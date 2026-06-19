// DockerEngineCard — live Docker engine status + enable/disable toggle,
// surfaced on the Docker Apps Maintenance tab so the engine state is
// visible where apps are managed (it was previously only on Server
// Settings → Docker App Marketplace). Reuses the existing endpoints:
//   GET  /admin/docker-apps/engine/status  → {installed, active, version}
//   GET/PATCH /admin/settings              → docker_marketplace_enabled
// Enabling installs Docker CE (~400 MB) in the background; the status
// query polls until the daemon reports active, mirroring the settings card.
import { App, Card, Space, Spin, Switch, Tag, Typography } from "antd";
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  ContainerOutlined,
} from "@icons";
import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { apiClient } from "../../../apiClient";

interface EngineStatus {
  installed: boolean;
  active: boolean;
  version?: string;
}

export function DockerEngineCard() {
  const { message } = App.useApp();
  const [enabled, setEnabled] = useState<boolean | null>(null);
  const [busy, setBusy] = useState(false);
  const [installing, setInstalling] = useState(false);

  const status = useQuery<EngineStatus>({
    queryKey: ["docker-engine-status"],
    queryFn: async () => {
      const { data } = await apiClient.get<EngineStatus>("/admin/docker-apps/engine/status");
      return data;
    },
    // Poll fast while an install/disable is in flight, idle otherwise.
    refetchInterval: () => (installing ? 4000 : false),
  });

  useEffect(() => {
    apiClient
      .get<{ docker_marketplace_enabled: boolean }>("/admin/settings")
      .then((r) => setEnabled(!!r.data.docker_marketplace_enabled))
      .catch((e) =>
        message.error(`Could not load Docker setting: ${e instanceof Error ? e.message : e}`),
      );
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Stop the install poll once the daemon comes up.
  useEffect(() => {
    if (installing && status.data?.installed && status.data?.active) {
      setInstalling(false);
      message.success("Docker engine installed + active.");
    }
  }, [installing, status.data, message]);

  const onToggle = async (value: boolean) => {
    setBusy(true);
    try {
      await apiClient.patch("/admin/settings", { docker_marketplace_enabled: value });
      setEnabled(value);
      if (value) {
        setInstalling(true);
        message.info("Enabling Docker — installing the engine can take a few minutes…");
      } else {
        message.warning(
          "Docker disabled. App data preserved under /var/lib/jabali/docker-apps/.",
        );
        status.refetch();
      }
    } catch (e) {
      message.error(e instanceof Error ? e.message : "Toggle failed");
    } finally {
      setBusy(false);
    }
  };

  const s = status.data;
  const engineTag = !s ? null : !s.installed ? (
    <Tag>Not installed</Tag>
  ) : s.active ? (
    <Tag icon={<CheckCircleOutlined />} color="success">Active</Tag>
  ) : (
    <Tag icon={<CloseCircleOutlined />} color="warning">Installed · stopped</Tag>
  );

  return (
    <Card
      title={
        <Space>
          <ContainerOutlined />
          Docker Engine
          {engineTag}
        </Space>
      }
      extra={
        enabled === null ? (
          <Spin size="small" />
        ) : (
          <Space>
            <Typography.Text type="secondary">
              {enabled ? "Enabled" : "Disabled"}
            </Typography.Text>
            <Switch checked={enabled} loading={busy || installing} onChange={onToggle} />
          </Space>
        )
      }
      style={{ marginBottom: 16 }}
    >
      <Space direction="vertical" size={8} style={{ width: "100%" }}>
        <Typography.Text type="secondary">
          The Docker daemon powering the app marketplace. Enabling installs Docker CE
          (~400 MB) on the host; disabling stops the daemon but keeps app data.
        </Typography.Text>
        <Space size={24} wrap>
          <span>
            <Typography.Text type="secondary">Installed: </Typography.Text>
            {s ? (
              s.installed ? <Tag color="success">Yes</Tag> : <Tag>No</Tag>
            ) : (
              <Spin size="small" />
            )}
          </span>
          <span>
            <Typography.Text type="secondary">Daemon: </Typography.Text>
            {s ? (
              s.active ? <Tag color="success">active</Tag> : <Tag color="default">inactive</Tag>
            ) : (
              <Spin size="small" />
            )}
          </span>
          <span>
            <Typography.Text type="secondary">Version: </Typography.Text>
            <Typography.Text>{s?.version || "—"}</Typography.Text>
          </span>
        </Space>
      </Space>
    </Card>
  );
}
