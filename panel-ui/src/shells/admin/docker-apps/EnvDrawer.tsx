// EnvDrawer — view / edit / regenerate an installed docker-app's environment.
//
// The catalog generates per-install secrets (admin passwords, DB passwords,
// tokens) into the container's .env but never surfaced them, so an operator
// couldn't find an app's admin credential. This drawer reveals the env,
// lets you edit a value, and regenerate a generated secret. Saving or
// regenerating re-renders the compose and recreates the container (brief
// downtime), so we warn before applying.
import { Alert, App, Button, Drawer, Input, Popconfirm, Space, Table, Tooltip, Typography } from "antd";
import { CopyOutlined, EyeInvisibleOutlined, EyeOutlined, ReloadOutlined } from "@ant-design/icons";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";

import { getEnv, putEnv, regenerateEnv, type EnvVar } from "./api";
import type { InstalledApp } from "./types";

interface Props {
  open: boolean;
  app: InstalledApp | null;
  onClose: () => void;
}

export const EnvDrawer = ({ open, app, onClose }: Props) => {
  const { message } = App.useApp();
  const qc = useQueryClient();
  const [edits, setEdits] = useState<Record<string, string>>({});
  const [revealed, setRevealed] = useState<Record<string, boolean>>({});

  const envQ = useQuery({
    queryKey: ["docker-app-env", app?.id],
    queryFn: () => getEnv(app!.id),
    enabled: open && !!app,
  });

  useEffect(() => {
    if (open) {
      setEdits({});
      setRevealed({});
    }
  }, [open, app?.id]);

  const save = useMutation({
    mutationFn: () => putEnv(app!.id, edits),
    onSuccess: () => {
      message.success("Environment saved — container recreated");
      setEdits({});
      void qc.invalidateQueries({ queryKey: ["docker-app-env", app?.id] });
      void qc.invalidateQueries({ queryKey: ["admin-docker-apps"] });
    },
    onError: (e: unknown) =>
      message.error(`Save failed: ${(e as { response?: { data?: { detail?: string } } })?.response?.data?.detail ?? e}`),
  });

  const regen = useMutation({
    mutationFn: (key: string) => regenerateEnv(app!.id, key),
    onSuccess: (res) => {
      message.success(`Regenerated ${res.key}`);
      setRevealed((s) => ({ ...s, [res.key]: true }));
      void qc.invalidateQueries({ queryKey: ["docker-app-env", app?.id] });
      void qc.invalidateQueries({ queryKey: ["admin-docker-apps"] });
    },
    onError: (e: unknown) =>
      message.error(`Regenerate failed: ${(e as { response?: { data?: { detail?: string } } })?.response?.data?.detail ?? e}`),
  });

  const copy = (v: string) => {
    void navigator.clipboard?.writeText(v);
    message.success("Copied");
  };

  const rows = envQ.data ?? [];
  const dirty = Object.keys(edits).length > 0;

  return (
    <Drawer
      title={`Environment — ${app?.name ?? ""}`}
      width={720}
      open={open}
      onClose={onClose}
      destroyOnClose
      extra={
        <Space>
          <Button onClick={onClose}>Close</Button>
          <Popconfirm
            title="Recreate the container?"
            description="Saving applies the new values by recreating the container (brief downtime)."
            okText="Save & recreate"
            disabled={!dirty}
            onConfirm={() => save.mutate()}
          >
            <Button type="primary" disabled={!dirty} loading={save.isPending}>
              Save ({Object.keys(edits).length})
            </Button>
          </Popconfirm>
        </Space>
      }
    >
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="Editing a value or regenerating a secret recreates the container to apply it."
      />
      <Table<EnvVar>
        size="small"
        rowKey="name"
        loading={envQ.isLoading}
        dataSource={rows}
        pagination={false}
        scroll={{ x: "max-content" }}
        columns={[
          {
            title: "Variable",
            dataIndex: "name",
            render: (n: string, r) => (
              <Space size={4}>
                <Typography.Text code>{n}</Typography.Text>
                {r.generated && <Typography.Text type="secondary">(generated)</Typography.Text>}
              </Space>
            ),
          },
          {
            title: "Value",
            dataIndex: "value",
            render: (v: string, r) => {
              const editing = r.name in edits;
              const shown = revealed[r.name] || !r.secret;
              return (
                <Space.Compact style={{ width: "100%" }}>
                  <Input
                    value={editing ? edits[r.name] : v}
                    type={shown ? "text" : "password"}
                    onChange={(e) => setEdits((s) => ({ ...s, [r.name]: e.target.value }))}
                  />
                  {r.secret && (
                    <Tooltip title={shown ? "Hide" : "Reveal"}>
                      <Button
                        icon={shown ? <EyeInvisibleOutlined /> : <EyeOutlined />}
                        onClick={() => setRevealed((s) => ({ ...s, [r.name]: !shown }))}
                      />
                    </Tooltip>
                  )}
                  <Tooltip title="Copy">
                    <Button icon={<CopyOutlined />} onClick={() => copy(editing ? edits[r.name] : v)} />
                  </Tooltip>
                </Space.Compact>
              );
            },
          },
          {
            title: "",
            key: "actions",
            width: 120,
            render: (_: unknown, r) =>
              r.generated ? (
                <Popconfirm
                  title={`Regenerate ${r.name}?`}
                  description="Mints a new secret and recreates the container."
                  okText="Regenerate"
                  onConfirm={() => regen.mutate(r.name)}
                >
                  <Button size="small" icon={<ReloadOutlined />} loading={regen.isPending && regen.variables === r.name}>
                    Regenerate
                  </Button>
                </Popconfirm>
              ) : null,
          },
        ]}
      />
    </Drawer>
  );
};
