// Tenant Docker Apps page (M49, GH #170). Catalog grid of tenant-installable
// apps + an install modal (loopback-only, attached to a domain you own) + a
// table of your installs with start/stop/restart/delete. Degrades to a clear
// "not enabled on this server" notice when the host has no userns-remap (the
// backend 403s docker_tenant_not_enabled).
import { useMemo, useState } from "react";
import {
  Alert,
  Button,
  Card,
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
  installApp,
  lifecycleAction,
  listCatalog,
  listInstalled,
} from "./api";

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
              <Space>
                {r.status === "stopped" ? (
                  <Button
                    size="small"
                    icon={<PlayCircleOutlined />}
                    onClick={() => lifecycle.mutate({ id: r.id, action: "start" })}
                  />
                ) : (
                  <Button
                    size="small"
                    icon={<PauseCircleOutlined />}
                    onClick={() => lifecycle.mutate({ id: r.id, action: "stop" })}
                  />
                )}
                <Button
                  size="small"
                  icon={<ReloadOutlined />}
                  onClick={() => lifecycle.mutate({ id: r.id, action: "restart" })}
                />
                <Button
                  size="small"
                  danger
                  icon={<DeleteOutlined />}
                  onClick={() =>
                    Modal.confirm({
                      title: `Delete "${r.name}"?`,
                      content: "This removes the container and its data.",
                      okText: "Delete",
                      okButtonProps: { danger: true },
                      onOk: () => remove.mutateAsync(r.id),
                    })
                  }
                />
              </Space>
            ),
          },
        ]}
      />

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
