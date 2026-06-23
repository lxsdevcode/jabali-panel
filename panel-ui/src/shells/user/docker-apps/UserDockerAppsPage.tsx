// Tenant Docker Apps page (M49, GH #170). Catalog grid of tenant-installable
// apps + an install modal (loopback-only, attached to a domain you own) + a
// table of your installs with start/stop/restart/delete. Degrades to a clear
// "not enabled on this server" notice when the host has no userns-remap (the
// backend 403s docker_tenant_not_enabled).
import { useMemo, useState } from "react";
import { RowActions } from "../../../components/RowActions";
import {
  Alert,
  Button,
  Card,
  Descriptions,
  Form,
  Input,
  Modal,
  Space,
  Table,
  Tag,
  Typography,
  notification,
} from "antd";
import {
  AppstoreOutlined,
  DeleteOutlined,
  FileTextOutlined,
  KeyOutlined,
  PauseCircleOutlined,
  PlayCircleOutlined,
  ReloadOutlined,
} from "@icons";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { AxiosError } from "axios";

import type { CatalogEntry, InstalledApp } from "../../admin/docker-apps/types";
import {
  catalogIconUrl,
  deleteApp,
  fetchEnv,
  fetchLogs,
  fetchUsage,
  installApp,
  lifecycleAction,
  listCatalog,
  listInstalled,
} from "./api";
import type { EnvVarView } from "./api";

const statusColor = (s: InstalledApp["status"]) =>
  s === "running"
    ? "green"
    : s === "failed"
      ? "red"
      : s === "installing" || s === "updating"
        ? "blue"
        : "default";

export const UserDockerAppsPage = () => {
  const qc = useQueryClient();
  const [installFor, setInstallFor] = useState<CatalogEntry | null>(null);
  const [logsFor, setLogsFor] = useState<InstalledApp | null>(null);
  const [credsFor, setCredsFor] = useState<InstalledApp | null>(null);

  const catalog = useQuery({ queryKey: ["user-docker-catalog"], queryFn: listCatalog });
  const installed = useQuery({
    queryKey: ["user-docker-installed"],
    queryFn: listInstalled,
    // Poll while anything is mid-install so the status tag flips live.
    refetchInterval: (q) =>
      (q.state.data ?? []).some((a) => a.status === "installing" || a.status === "updating")
        ? 8000
        : false,
  });

  const usage = useQuery({ queryKey: ["user-docker-usage"], queryFn: fetchUsage, retry: false });

  // The host-flag 403 surfaces on the installed-list query; treat it as the
  // "tenant docker disabled on this server" state.
  const disabled =
    (installed.error as AxiosError | undefined)?.response?.status === 403 ||
    (catalog.error as AxiosError | undefined)?.response?.status === 403;

  const lifecycle = useMutation({
    mutationFn: ({ id, action }: { id: string; action: "start" | "stop" | "restart" }) =>
      lifecycleAction(id, action),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["user-docker-installed"] }),
    onError: (e: AxiosError<{ detail?: string }>) =>
      notification.error({ message: "Action failed", description: e.response?.data?.detail }),
  });
  const remove = useMutation({
    mutationFn: (id: string) => deleteApp(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["user-docker-installed"] }),
  });

  const rows = useMemo(() => installed.data ?? [], [installed.data]);

  if (disabled) {
    return (
      <div>
        <Typography.Title level={3} style={{ marginTop: 0 }}>
          <AppstoreOutlined /> Docker Apps
        </Typography.Title>
        <Alert
          type="info"
          showIcon
          message="Docker apps are not enabled on this server"
          description="Ask your administrator to enable tenant Docker apps for your hosting package."
        />
      </div>
    );
  }

  return (
    <div>
      <Typography.Title level={3} style={{ marginTop: 0 }}>
        <AppstoreOutlined /> Docker Apps
      </Typography.Title>

      {usage.data?.over_quota && (
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 16 }}
          message="Docker apps are over your disk quota"
          description="Your installed apps exceed your package disk allowance. Delete an app or ask your administrator to raise the quota."
        />
      )}
      <Typography.Title level={5}>Available apps</Typography.Title>
      <Space wrap size={[16, 16]} style={{ marginBottom: 24 }}>
        {(catalog.data ?? []).map((e) => (
          <Card
            key={e.slug}
            size="small"
            style={{ width: 240 }}
            styles={{ body: { display: "flex", flexDirection: "column", gap: 8 } }}
          >
            <Space>
              <img src={catalogIconUrl(e.slug)} alt="" width={32} height={32} />
              <div>
                <Typography.Text strong>{e.name}</Typography.Text>
                <br />
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                  v{e.version}
                </Typography.Text>
              </div>
            </Space>
            <Typography.Paragraph
              type="secondary"
              ellipsis={{ rows: 3 }}
              style={{ fontSize: 12, margin: 0, minHeight: 48 }}
            >
              {e.description}
            </Typography.Paragraph>
            <Button type="primary" size="small" block onClick={() => setInstallFor(e)}>
              Install
            </Button>
          </Card>
        ))}
        {catalog.data?.length === 0 && (
          <Typography.Text type="secondary">No apps available to install.</Typography.Text>
        )}
      </Space>

      <Typography.Title level={5}>Your apps</Typography.Title>
      <Table<InstalledApp>
        rowKey="id"
        size="small"
        loading={installed.isLoading}
        dataSource={rows}
        pagination={false}
        scroll={{ x: "max-content" }}
        columns={[
          { title: "Name", dataIndex: "name" },
          { title: "App", dataIndex: "slug" },
          {
            title: "Domain",
            dataIndex: "domain",
            render: (d: string | undefined) =>
              d ? (
                <a href={`https://${d}/`} target="_blank" rel="noreferrer">
                  {d}
                </a>
              ) : (
                "—"
              ),
          },
          {
            title: "Status",
            dataIndex: "status",
            render: (s: InstalledApp["status"]) => <Tag color={statusColor(s)}>{s}</Tag>,
          },
          {
            title: "Actions",
            key: "actions",
            render: (_, r) => (
              <RowActions
                actions={[
                  { key: "start", label: "Start", icon: <PlayCircleOutlined />, hidden: r.status !== "stopped", onClick: () => lifecycle.mutate({ id: r.id, action: "start" }) },
                  { key: "stop", label: "Stop", icon: <PauseCircleOutlined />, hidden: r.status === "stopped", onClick: () => lifecycle.mutate({ id: r.id, action: "stop" }) },
                  { key: "restart", label: "Restart", icon: <ReloadOutlined />, onClick: () => lifecycle.mutate({ id: r.id, action: "restart" }) },
                  { key: "logs", label: "Logs", icon: <FileTextOutlined />, onClick: () => setLogsFor(r) },
                  { key: "creds", label: "Credentials", icon: <KeyOutlined />, onClick: () => setCredsFor(r) },
                  {
                    key: "delete",
                    label: "Delete",
                    icon: <DeleteOutlined />,
                    danger: true,
                    onClick: () => { void remove.mutateAsync(r.id); },
                    confirm: { title: `Delete "${r.name}"?`, description: "This removes the container and its data.", okText: "Delete" },
                  },
                ]}
              />
            ),
          },
        ]}
      />

      <LogsModal app={logsFor} onClose={() => setLogsFor(null)} />
      <CredentialsModal app={credsFor} onClose={() => setCredsFor(null)} />

      <InstallModal
        entry={installFor}
        onClose={() => setInstallFor(null)}
        onInstalled={() => {
          setInstallFor(null);
          qc.invalidateQueries({ queryKey: ["user-docker-installed"] });
        }}
      />
    </div>
  );
};

const InstallModal = ({
  entry,
  onClose,
  onInstalled,
}: {
  entry: CatalogEntry | null;
  onClose: () => void;
  onInstalled: () => void;
}) => {
  const [form] = Form.useForm<{ name: string; domain: string }>();
  const install = useMutation({
    mutationFn: (v: { name: string; domain: string }) =>
      installApp({ slug: entry!.slug, name: v.name, domain: v.domain }),
    onSuccess: () => {
      notification.success({ message: "Install started" });
      form.resetFields();
      onInstalled();
    },
    onError: (e: AxiosError<{ detail?: string; error?: string }>) =>
      notification.error({
        message: "Install failed",
        description: e.response?.data?.detail || e.response?.data?.error,
      }),
  });

  return (
    <Modal
      open={!!entry}
      title={entry ? `Install ${entry.name}` : ""}
      onCancel={onClose}
      okText="Install"
      confirmLoading={install.isPending}
      onOk={() => form.validateFields().then((v) => install.mutate(v))}
    >
      <Form form={form} layout="vertical">
        <Form.Item
          name="name"
          label="Install name"
          rules={[
            { required: true, message: "Required" },
            { pattern: /^[a-z0-9-]{1,32}$/, message: "lowercase letters, digits, hyphens" },
          ]}
        >
          <Input placeholder="my-notes" />
        </Form.Item>
        <Form.Item
          name="domain"
          label="Domain"
          extra="A domain you own (or a new hostname). The app is served here over HTTPS."
          rules={[{ required: true, message: "Required" }]}
        >
          <Input placeholder="notes.example.com" />
        </Form.Item>
      </Form>
    </Modal>
  );
};

const LogsModal = ({ app, onClose }: { app: InstalledApp | null; onClose: () => void }) => {
  const q = useQuery({
    queryKey: ["user-docker-logs", app?.id],
    queryFn: () => fetchLogs(app!.id),
    enabled: !!app,
  });
  return (
    <Modal
      open={!!app}
      title={app ? `Logs — ${app.name}` : ""}
      onCancel={onClose}
      onOk={onClose}
      width={760}
      footer={null}
    >
      {q.isLoading ? (
        "Loading…"
      ) : (
        <pre
          style={{
            maxHeight: 460,
            overflow: "auto",
            whiteSpace: "pre-wrap",
            wordBreak: "break-word",
            margin: 0,
            fontSize: 12,
          }}
        >
          {q.data?.logs || "(no output)"}
        </pre>
      )}
    </Modal>
  );
};

const CredentialsModal = ({ app, onClose }: { app: InstalledApp | null; onClose: () => void }) => {
  const q = useQuery({
    queryKey: ["user-docker-env", app?.id],
    queryFn: () => fetchEnv(app!.id),
    enabled: !!app,
  });
  return (
    <Modal
      open={!!app}
      title={app ? `Credentials — ${app.name}` : ""}
      onCancel={onClose}
      onOk={onClose}
      width={640}
      footer={null}
    >
      <Typography.Paragraph type="secondary" style={{ marginTop: 0 }}>
        Generated secrets for this install (admin password, DB password, keys).
      </Typography.Paragraph>
      <Descriptions size="small" column={1} bordered>
        {(q.data ?? []).map((e: EnvVarView) => (
          <Descriptions.Item key={e.name} label={e.name}>
            <Typography.Text copyable={!!e.value} code>
              {e.value || "—"}
            </Typography.Text>
          </Descriptions.Item>
        ))}
      </Descriptions>
    </Modal>
  );
};
