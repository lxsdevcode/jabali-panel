// AdminDockerAppsPage — landing page for the M48 marketplace.
// Two tabs: Catalog (browse + install) and Installed (lifecycle).
import { App, Avatar, Button, Card, Col, Popconfirm, Row, Space, Table, Tabs, Tag, Tooltip, Typography } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import {
  AppstoreOutlined,
  PlayCircleOutlined,
  PauseCircleOutlined,
  ReloadOutlined,
  DeleteOutlined,
  SyncOutlined,
  FileTextOutlined,
  CodeOutlined,
} from "@icons";

import { deleteApp, lifecycleAction, listCatalog, listInstalled, updateApp } from "./api";
import type { CatalogEntry, InstalledApp } from "./types";
import { InstallDrawer } from "./InstallDrawer";
import { LogsDrawer } from "./LogsDrawer";
import { ExecDrawer } from "./ExecDrawer";

const STATUS_COLOR: Record<string, string> = {
  pending: "default",
  installing: "blue",
  running: "green",
  stopped: "orange",
  failed: "red",
  updating: "blue",
  rolling_back: "purple",
  deleted: "default",
};

export const AdminDockerAppsPage = () => {
  const { message } = App.useApp();
  const qc = useQueryClient();
  const [installEntry, setInstallEntry] = useState<CatalogEntry | null>(null);
  const [logsAppId, setLogsAppId] = useState<string | null>(null);
  const [execAppId, setExecAppId] = useState<string | null>(null);

  const catalog = useQuery({
    queryKey: ["docker-apps-catalog"],
    queryFn: listCatalog,
  });
  const installed = useQuery({
    queryKey: ["docker-apps-installed"],
    queryFn: listInstalled,
    refetchInterval: 8000, // poll while installs are in flight
  });

  const lifecycle = useMutation({
    mutationFn: async ({ id, action }: { id: string; action: "start" | "stop" | "restart" | "rebuild" }) =>
      lifecycleAction(id, action),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["docker-apps-installed"] }),
    onError: (e: unknown) => message.error(e instanceof Error ? e.message : "Action failed"),
  });

  const updateImage = useMutation({
    mutationFn: async (id: string) => updateApp(id),
    onSuccess: (r) => {
      if (r.outcome === "rolled_back") {
        message.warning(r.detail ? `Rolled back: ${r.detail}` : "Update failed; rolled back to previous image");
      } else {
        message.success("Updated");
      }
      qc.invalidateQueries({ queryKey: ["docker-apps-installed"] });
    },
    onError: (e: unknown) => message.error(e instanceof Error ? e.message : "Update failed"),
  });

  const remove = useMutation({
    mutationFn: async (id: string) => deleteApp(id, false),
    onSuccess: () => {
      message.success("Uninstalled");
      qc.invalidateQueries({ queryKey: ["docker-apps-installed"] });
    },
    onError: (e: unknown) => message.error(e instanceof Error ? e.message : "Delete failed"),
  });

  return (
    <div>
      <Space style={{ marginBottom: 16, width: "100%", justifyContent: "space-between" }}>
        <Typography.Title level={3} style={{ margin: 0 }}>
          <AppstoreOutlined /> Docker Apps
        </Typography.Title>
        <Typography.Text type="secondary">
          {installed.data?.length ?? 0} installed - {catalog.data?.length ?? 0} in catalog
        </Typography.Text>
      </Space>

      <Tabs
        defaultActiveKey="catalog"
        items={[
          {
            key: "catalog",
            label: "Catalog",
            children: (
              <Row gutter={[16, 16]}>
                {(catalog.data ?? []).map((e) => (
                  <Col key={e.slug} xs={24} sm={12} md={8}>
                    <Card
                      hoverable
                      onClick={() => setInstallEntry(e)}
                      actions={[<Button type="link" key="install">Install</Button>]}
                    >
                      <Card.Meta
                        avatar={<Avatar style={{ backgroundColor: "#f0f5ff" }}>{e.name[0]}</Avatar>}
                        title={
                          <Space>
                            {e.name}
                            <Tag>{e.version}</Tag>
                          </Space>
                        }
                        description={
                          <Tooltip title={e.description}>
                            <span style={{ display: "-webkit-box", WebkitLineClamp: 2, WebkitBoxOrient: "vertical", overflow: "hidden" }}>
                              {e.description}
                            </span>
                          </Tooltip>
                        }
                      />
                    </Card>
                  </Col>
                ))}
              </Row>
            ),
          },
          {
            key: "installed",
            label: "Installed",
            children: (
              <Table<InstalledApp>
                rowKey="id"
                size="small"
                loading={installed.isLoading}
                dataSource={installed.data ?? []}
                pagination={{ pageSize: 25 }}
                columns={[
                  {
                    title: "Name",
                    dataIndex: "name",
                    render: (n, r) => (
                      <Space direction="vertical" size={0}>
                        <Typography.Text strong>{n}</Typography.Text>
                        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                          {r.slug} @ {r.catalog_version}
                        </Typography.Text>
                      </Space>
                    ),
                  },
                  {
                    title: "Status",
                    dataIndex: "status",
                    width: 130,
                    render: (s, r) => (
                      <Space size={4}>
                        <Tag color={STATUS_COLOR[s] || "default"}>{s}</Tag>
                        {(s === "installing" || s === "updating" || s === "rolling_back") && <SyncOutlined spin />}
                        {r.last_error && (
                          <Tooltip title={r.last_error}>
                            <Tag color="red">err</Tag>
                          </Tooltip>
                        )}
                      </Space>
                    ),
                  },
                  {
                    title: "Ports",
                    dataIndex: "ports",
                    render: (_, r) =>
                      (r.ports ?? []).map((p) => (
                        <Tag key={p.id} style={{ marginRight: 4 }}>
                          {p.port_name}: {p.bind_interface}:{p.host_port}/{p.protocol}
                        </Tag>
                      )),
                  },
                  {
                    title: "Limits",
                    width: 150,
                    render: (_, r) => (
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                        cpu={r.cpu_limit ?? "-"} mem={r.memory_limit ?? "-"} pids={r.pids_limit ?? "-"}
                      </Typography.Text>
                    ),
                  },
                  {
                    title: "Actions",
                    width: 240,
                    render: (_, r) => (
                      <Space size={4}>
                        <Button
                          size="small"
                          icon={<PlayCircleOutlined />}
                          onClick={() => lifecycle.mutate({ id: r.id, action: "start" })}
                          disabled={r.status === "running"}
                        >
                          Start
                        </Button>
                        <Button
                          size="small"
                          icon={<PauseCircleOutlined />}
                          onClick={() => lifecycle.mutate({ id: r.id, action: "stop" })}
                          disabled={r.status !== "running"}
                        >
                          Stop
                        </Button>
                        <Button
                          size="small"
                          icon={<ReloadOutlined />}
                          onClick={() => lifecycle.mutate({ id: r.id, action: "restart" })}
                        >
                          Restart
                        </Button>
                        <Button
                          size="small"
                          icon={<SyncOutlined />}
                          loading={updateImage.isPending && updateImage.variables === r.id}
                          onClick={() => updateImage.mutate(r.id)}
                          disabled={r.update_mode === "manual" ? false : false}
                          title="Pull latest image with rollback on failure"
                        >
                          Update
                        </Button>
                        <Button
                          size="small"
                          icon={<FileTextOutlined />}
                          onClick={() => setLogsAppId(r.id)}
                          title="Tail container logs"
                        >
                          Logs
                        </Button>
                        <Button
                          size="small"
                          icon={<CodeOutlined />}
                          onClick={() => setExecAppId(r.id)}
                          title="Run a command inside the container"
                        >
                          Exec
                        </Button>
                        <Popconfirm
                          title={`Uninstall ${r.name}?`}
                          description="Volumes will be purged. This cannot be undone."
                          okText="Uninstall"
                          okButtonProps={{ danger: true }}
                          onConfirm={() => remove.mutate(r.id)}
                        >
                          <Button size="small" danger icon={<DeleteOutlined />} />
                        </Popconfirm>
                      </Space>
                    ),
                  },
                ]}
              />
            ),
          },
        ]}
      />

      <InstallDrawer
        open={installEntry !== null}
        entry={installEntry}
        onClose={() => setInstallEntry(null)}
      />
      <LogsDrawer
        open={logsAppId !== null}
        appId={logsAppId}
        onClose={() => setLogsAppId(null)}
      />
      <ExecDrawer
        open={execAppId !== null}
        appId={execAppId}
        onClose={() => setExecAppId(null)}
      />
    </div>
  );
};
