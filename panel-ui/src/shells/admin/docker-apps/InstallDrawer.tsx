// InstallDrawer — picks a catalog entry and submits an install
// request. Renders one editable row per catalog-declared port so the
// admin can flip Enabled / Bind / Host port / Reverse-proxy before
// committing. See ADR-0116 Decision 5.
import { Alert, App, Button, Drawer, Form, Input, InputNumber, Select, Space, Switch, Table, Tag, Typography } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";

import { apiClient } from "../../../apiClient";

import { installApp } from "./api";
import type { CatalogEntry, InstallPortOverride, InstallRequest } from "./types";

interface Props {
  open: boolean;
  entry: CatalogEntry | null;
  onClose: () => void;
}

interface PortRow extends InstallPortOverride {
  // catalog metadata for read-only columns
  container_port: number;
  protocol: "tcp" | "udp";
}

export const InstallDrawer = ({ open, entry, onClose }: Props) => {
  const { message } = App.useApp();
  const qc = useQueryClient();
  const destsQ = useQuery({
    queryKey: ["admin-backup-destinations-for-docker-app"],
    queryFn: async () => {
      const { data } = await apiClient.get<{ data?: { id: string; name: string; kind: string }[] }>(
        "/admin/backup-destinations?pageSize=100",
      );
      return data.data ?? [];
    },
  });
  const [form] = Form.useForm();
  const [ports, setPorts] = useState<PortRow[]>([]);

  useEffect(() => {
    if (!entry) {
      setPorts([]);
      return;
    }
    // Seed port rows from the catalog defaults whenever a new entry
    // is chosen (or the drawer reopens).
    setPorts(
      entry.ports.map((p) => ({
        name: p.name,
        enabled: p.default_enabled,
        bind_interface: p.default_bind,
        host_port: 0,
        reverse_proxy: p.default_reverse_proxy,
        container_port: p.container_port,
        protocol: p.protocol,
      })),
    );
    form.resetFields();
    form.setFieldsValue({
      slug: entry.slug,
      cpu_limit: entry.resources.cpu,
      memory_limit: entry.resources.memory,
      pids_limit: entry.resources.pids,
      update_mode: entry.update_mode,
    });
  }, [entry, form]);

  const install = useMutation({
    mutationFn: async (req: InstallRequest) => installApp(req),
    onSuccess: () => {
      message.success("Install dispatched");
      qc.invalidateQueries({ queryKey: ["docker-apps-installed"] });
      onClose();
    },
    onError: (e: unknown) =>
      message.error(
        e instanceof Error
          ? e.message
          : "Install failed; see server logs",
      ),
  });

  const handleFinish = (values: { slug: string; name: string; domain?: string; update_mode: "manual" | "auto"; cpu_limit: string; memory_limit: string; pids_limit?: number; backup_destination_id?: string }) => {
    if (!entry) return;
    install.mutate({
      slug: entry.slug,
      name: values.name,
      domain: values.domain,
      update_mode: values.update_mode,
      cpu_limit: values.cpu_limit,
      memory_limit: values.memory_limit,
      pids_limit: values.pids_limit,
      ports: ports.map(({ container_port: _cp, protocol: _proto, ...rest }) => rest),
      ...(values.backup_destination_id
        ? { backup_destination_id: values.backup_destination_id }
        : {}),
    });
  };

  const portsTable = useMemo(
    () => (
      <Table<PortRow>
        size="small"
        rowKey="name"
        dataSource={ports}
        pagination={false}
        columns={[
          { title: "Port", dataIndex: "name", width: 100 },
          {
            title: "Container",
            render: (_, row) => `${row.container_port}/${row.protocol}`,
            width: 100,
          },
          {
            title: "Enabled",
            dataIndex: "enabled",
            width: 90,
            render: (_, row) => (
              <Switch
                size="small"
                checked={row.enabled}
                onChange={(v) =>
                  setPorts((s) => s.map((p) => (p.name === row.name ? { ...p, enabled: v } : p)))
                }
              />
            ),
          },
          {
            title: "Bind",
            dataIndex: "bind_interface",
            width: 130,
            render: (_, row) => (
              <Select
                size="small"
                value={row.bind_interface}
                style={{ width: "100%" }}
                onChange={(v) =>
                  setPorts((s) => s.map((p) => (p.name === row.name ? { ...p, bind_interface: v } : p)))
                }
                options={[
                  { value: "loopback", label: "loopback" },
                  { value: "public", label: "public IP" },
                ]}
              />
            ),
          },
          {
            title: "Host port",
            dataIndex: "host_port",
            width: 120,
            render: (_, row) => (
              <InputNumber
                size="small"
                placeholder="auto"
                value={row.host_port || undefined}
                min={1024}
                max={65535}
                onChange={(v) =>
                  setPorts((s) => s.map((p) => (p.name === row.name ? { ...p, host_port: v ?? 0 } : p)))
                }
              />
            ),
          },
          {
            title: "Reverse proxy",
            dataIndex: "reverse_proxy",
            width: 110,
            render: (_, row) => (
              <Switch
                size="small"
                checked={row.reverse_proxy}
                disabled={row.bind_interface !== "loopback" || row.protocol === "udp"}
                onChange={(v) =>
                  setPorts((s) => s.map((p) => (p.name === row.name ? { ...p, reverse_proxy: v } : p)))
                }
              />
            ),
          },
        ]}
      />
    ),
    [ports],
  );

  return (
    <Drawer
      open={open}
      onClose={onClose}
      title={entry ? `Install ${entry.name}` : "Install"}
      width={720}
      destroyOnClose
      extra={
        <Space>
          <Button onClick={onClose}>Cancel</Button>
          <Button type="primary" loading={install.isPending} onClick={() => form.submit()}>
            Install
          </Button>
        </Space>
      }
    >
      {entry && (
        <Form layout="vertical" form={form} onFinish={handleFinish}>
          <Alert
            type="info"
            message={`${entry.name} ${entry.version}`}
            description={entry.description}
            style={{ marginBottom: 16 }}
            showIcon
          />
          <Form.Item label="Slug" name="slug">
            <Input disabled />
          </Form.Item>
          <Form.Item
            label="Install name"
            name="name"
            rules={[
              { required: true, message: "Name is required" },
              { pattern: /^[a-z0-9-]{1,32}$/, message: "Lowercase letters, digits, dashes; up to 32 chars" },
            ]}
            tooltip="Per-install identifier. Used as the docker compose project name and the data root."
          >
            <Input placeholder="e.g. vault-prod" />
          </Form.Item>
          <Form.Item
            label="Domain"
            name="domain"
            tooltip="Hostname to attach to the loopback-bound port via nginx. Optional in Phase 5; required once you wire reverse-proxy (Phase 6)."
          >
            <Input placeholder="vault.example.com" />
          </Form.Item>
          <Space size="middle" style={{ width: "100%" }}>
            <Form.Item label="Update mode" name="update_mode" style={{ flex: 1 }}>
              <Select
                options={[
                  { value: "manual", label: "Manual" },
                  { value: "auto", label: "Auto + rollback" },
                ]}
              />
            </Form.Item>
            <Form.Item label="CPU" name="cpu_limit" style={{ flex: 1 }}>
              <Input placeholder="0.5" />
            </Form.Item>
            <Form.Item label="Memory" name="memory_limit" style={{ flex: 1 }}>
              <Input placeholder="256m" />
            </Form.Item>
            <Form.Item label="PIDs" name="pids_limit" style={{ flex: 1 }}>
              <InputNumber min={1} max={65535} />
            </Form.Item>
          </Space>

          <Form.Item
            label="Backup destination"
            name="backup_destination_id"
            tooltip="Pick a destination from Server Settings -> Backups. Leave empty to fall back to JABALI_RESTIC_REPO env vars (Phase 8.1)."
          >
            <Select
              allowClear
              placeholder="Inherit env-var fallback"
              options={(destsQ.data ?? []).map((d) => ({
                value: d.id,
                label: `${d.name} (${d.kind})`,
              }))}
              loading={destsQ.isLoading}
            />
          </Form.Item>

          <Typography.Title level={5} style={{ marginTop: 8 }}>
            Ports
          </Typography.Title>
          <Typography.Paragraph type="secondary" style={{ marginTop: 0 }}>
            Catalog declares the universe. Disable ports you don't need. <Tag>Bind=loopback</Tag>
            puts the port behind nginx (TLS + CrowdSec). <Tag>Bind=public</Tag> binds to a panel-managed IP
            directly (UDP, SSH, etc).
          </Typography.Paragraph>
          {portsTable}
        </Form>
      )}
    </Drawer>
  );
};
