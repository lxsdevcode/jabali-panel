// AdminDockerAppsPage — landing page for the M48 marketplace.
// Two tabs: Catalog (browse + install) and Installed (lifecycle).
import { App, Avatar, Button, Card, Col, Dropdown, Empty, Modal, Row, Space, Statistic, Table, Tabs, Tag, Tooltip, Typography } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import {
  AppstoreOutlined,
  LayoutPanelLeftOutlined,
  PlayCircleOutlined,
  PauseCircleOutlined,
  ReloadOutlined,
  DeleteOutlined,
  SyncOutlined,
  FileTextOutlined,
  CodeOutlined,
  SaveOutlined,
  EditOutlined,
  MoreOutlined,
} from "@icons";

import { deleteApp, lifecycleAction, listCatalog, listInstalled, updateApp } from "./api";
import type { CatalogEntry, InstalledApp } from "./types";
import { InstallDrawer } from "./InstallDrawer";
import { LogsDrawer } from "./LogsDrawer";
import { ExecDrawer } from "./ExecDrawer";
import { BackupsDrawer } from "./BackupsDrawer";
import { EditDrawer } from "./EditDrawer";
import { MaintenanceTab } from "./MaintenanceTab";

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
  const [editApp, setEditApp] = useState<InstalledApp | null>(null);
  const [activeTab, setActiveTab] = useState<string>("installed");
  const [backupsAppId, setBackupsAppId] = useState<string | null>(null);

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
          <LayoutPanelLeftOutlined /> Docker Apps
        </Typography.Title>
        <Typography.Text type="secondary">
          {installed.data?.length ?? 0} installed
        </Typography.Text>
      </Space>

      <Tabs
        activeKey={activeTab}
        onChange={setActiveTab}
        items={[
          {
            key: "installed",
            label: "Installed",
            children: (
              <div>
                {(() => {
                  const rows = installed.data ?? [];
                  const installedCount = rows.length;
                  const runningCount = rows.filter(r => r.status === "running").length;
                  const stoppedCount = rows.filter(r => r.status === "stopped").length;
                  const updateCount = rows.filter(r => r.available_digest && r.available_digest !== r.image_sha).length;
                  return (
                    <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
                      <Col xs={12} sm={6}>
                        <Card size="small">
                          <Statistic
                            title={<Space><AppstoreOutlined />Installed Apps</Space>}
                            value={installedCount}
                          />
                        </Card>
                      </Col>
                      <Col xs={12} sm={6}>
                        <Card size="small">
                          <Statistic
                            title={<Space><SyncOutlined />Updates</Space>}
                            value={updateCount}
                            valueStyle={{ color: updateCount > 0 ? "#cf1322" : undefined }}
                            suffix={updateCount > 0 ? <Typography.Text type="warning" style={{ fontSize: 12 }}>needs attention</Typography.Text> : null}
                          />
                        </Card>
                      </Col>
                      <Col xs={12} sm={6}>
                        <Card size="small">
                          <Statistic
                            title={<Space><PlayCircleOutlined />Running</Space>}
                            value={runningCount}
                            valueStyle={{ color: runningCount > 0 ? "#3f8600" : undefined }}
                            suffix={installedCount > 0 ? <Typography.Text type="secondary" style={{ fontSize: 12 }}>/ {installedCount}</Typography.Text> : null}
                          />
                        </Card>
                      </Col>
                      <Col xs={12} sm={6}>
                        <Card size="small">
                          <Statistic
                            title={<Space><PauseCircleOutlined />Stopped</Space>}
                            value={stoppedCount}
                            valueStyle={{ color: stoppedCount > 0 ? "#d46b08" : undefined }}
                            suffix={installedCount > 0 ? <Typography.Text type="secondary" style={{ fontSize: 12 }}>/ {installedCount}</Typography.Text> : null}
                          />
                        </Card>
                      </Col>
                    </Row>
                  );
                })()}
              <Table<InstalledApp>
                rowKey="id"
                size="small"
                loading={installed.isLoading}
                dataSource={installed.data ?? []}
                pagination={{ pageSize: 25 }}
                locale={{
                  emptyText: (
                    <Empty
                      image={Empty.PRESENTED_IMAGE_SIMPLE}
                      description="No installed apps yet"
                    >
                      <Button type="primary" onClick={() => setActiveTab("catalog")}>
                        Browse Catalog
                      </Button>
                    </Empty>
                  ),
                }}
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
                    width: 170,
                    render: (s, r) => (
                      <Space size={4}>
                        <Tag color={STATUS_COLOR[s] || "default"}>{s}</Tag>
                        {(s === "installing" || s === "updating" || s === "rolling_back") && <SyncOutlined spin />}
                        {r.last_error && (
                          <Tooltip title={r.last_error}>
                            <Tag color="red">err</Tag>
                          </Tooltip>
                        )}
                        {r.available_digest && r.available_digest !== (r.image_sha ?? "") && (
                          <Tooltip title={`Upstream digest moved to ${r.available_digest.slice(0, 19)}...`}>
                            <Tag color="purple">update available</Tag>
                          </Tooltip>
                        )}
                      </Space>
                    ),
                  },
                  {
                    title: "Domain",
                    dataIndex: "domain",
                    width: 220,
                    render: (_, r) => (r.domain
                      ? <Tag color="blue">{r.domain}</Tag>
                      : <Typography.Text type="secondary">—</Typography.Text>),
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
                    width: 180,
                    align: "right" as const,
                    render: (_, r) => {
                      const running = r.status === "running";
                      const confirmDelete = () =>
                        Modal.confirm({
                          title: `Uninstall ${r.name}?`,
                          content: "Volumes will be purged. This cannot be undone.",
                          okText: "Uninstall",
                          okButtonProps: { danger: true },
                          onOk: () => remove.mutate(r.id),
                        });
                      return (
                        <Space size={4}>
                          {running ? (
                            <Tooltip title="Stop">
                              <Button
                                size="small"
                                icon={<PauseCircleOutlined />}
                                onClick={() => lifecycle.mutate({ id: r.id, action: "stop" })}
                              />
                            </Tooltip>
                          ) : (
                            <Tooltip title="Start">
                              <Button
                                size="small"
                                icon={<PlayCircleOutlined />}
                                onClick={() => lifecycle.mutate({ id: r.id, action: "start" })}
                              />
                            </Tooltip>
                          )}
                          <Tooltip title="Restart">
                            <Button
                              size="small"
                              icon={<ReloadOutlined />}
                              onClick={() => lifecycle.mutate({ id: r.id, action: "restart" })}
                            />
                          </Tooltip>
                          <Tooltip title={running ? "Stop the container before editing" : "Edit"}>
                            <Button
                              size="small"
                              icon={<EditOutlined />}
                              disabled={running}
                              onClick={() => setEditApp(r)}
                            />
                          </Tooltip>
                          <Dropdown
                            trigger={["click"]}
                            menu={{
                              items: [
                                {
                                  key: "update",
                                  icon: <SyncOutlined />,
                                  label: "Update",
                                  onClick: () => updateImage.mutate(r.id),
                                },
                                {
                                  key: "logs",
                                  icon: <FileTextOutlined />,
                                  label: "Logs",
                                  onClick: () => setLogsAppId(r.id),
                                },
                                {
                                  key: "exec",
                                  icon: <CodeOutlined />,
                                  label: "Exec",
                                  onClick: () => setExecAppId(r.id),
                                },
                                {
                                  key: "backups",
                                  icon: <SaveOutlined />,
                                  label: "Backups",
                                  onClick: () => setBackupsAppId(r.id),
                                },
                                { type: "divider" as const, key: "d1" },
                                {
                                  key: "delete",
                                  icon: <DeleteOutlined />,
                                  label: "Uninstall",
                                  danger: true,
                                  onClick: confirmDelete,
                                },
                              ],
                            }}
                          >
                            <Button size="small" icon={<MoreOutlined />} />
                          </Dropdown>
                        </Space>
                      );
                    },
                  },
                ]}
              />
              </div>
            ),
          },
          {
            key: "catalog",
            label: "Catalog",
            children: (
              <div
                style={{
                  columnGap: 16,
                  // CSS shorthand: as many columns of >=320px as the
                  // viewport fits. Works at mobile (1 col), tablet (2),
                  // desktop (3+) without media queries.
                  columns: "320px",
                }}
                className="docker-apps-catalog-masonry"
              >
                {(catalog.data ?? []).map((e) => (
                  <div
                    key={e.slug}
                    style={{ breakInside: "avoid", marginBottom: 16, display: "inline-block", width: "100%" }}
                  >
                    <Card
                      hoverable
                      onClick={() => setInstallEntry(e)}
                      styles={{ body: { padding: 16 } }}
                      actions={[<Button type="link" key="install">Install</Button>]}
                    >
                      <Card.Meta
                        avatar={
                          <Avatar
                            shape="square"
                            size={40}
                            src={`/api/v1/admin/docker-apps/catalog/${e.slug}/icon`}
                            style={{ backgroundColor: "transparent", color: "#1f1f1f" }}
                          >
                            {e.name[0]}
                          </Avatar>
                        }
                        title={
                          <Space size={6} wrap>
                            <span>{e.name}</span>
                            <Tag style={{ marginInlineEnd: 0 }}>{e.version}</Tag>
                          </Space>
                        }
                        description={
                          <div style={{ whiteSpace: "pre-line" }}>
                            {e.description}
                          </div>
                        }
                      />
                    </Card>
                  </div>
                ))}
              </div>
            ),
          },
          {
            key: "maintenance",
            label: "Maintenance",
            children: <MaintenanceTab />,
          }

        ]}
      />

      <InstallDrawer
        open={installEntry !== null}
        entry={installEntry}
        onClose={() => setInstallEntry(null)}
        onInstalled={() => setActiveTab("installed")}
      />
      <LogsDrawer
        open={logsAppId !== null}
        appId={logsAppId}
        appName={(installed.data ?? []).find((a) => a.id === logsAppId)?.name}
        lastError={(installed.data ?? []).find((a) => a.id === logsAppId)?.last_error}
        onClose={() => setLogsAppId(null)}
      />
      <ExecDrawer
        open={execAppId !== null}
        appId={execAppId}
        onClose={() => setExecAppId(null)}
      />
      <EditDrawer
        open={editApp !== null}
        app={editApp}
        onClose={() => setEditApp(null)}
      />
      <BackupsDrawer
        open={backupsAppId !== null}
        appId={backupsAppId}
        onClose={() => setBackupsAppId(null)}
      />
    </div>
  );
};
