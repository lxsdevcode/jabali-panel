// MyProfileBackupCard — user-shell self-backup card. Generate full
// backup of the caller's account; list recent self-backups; download
// when a row is succeeded. Mirrors AdminBackupsPage data shape but
// scoped via /me/backups (auth-gated to caller's user_id).
import { Button, Card, Grid, Select, Space, Table, Tag, Typography, message } from "antd";
import { getActAs } from "../../impersonation";
import { shortDateTime } from "../../utils/datetime";
import { RowActions } from "../../components/RowActions";
import { DeleteOutlined, DownloadOutlined, ReloadOutlined, SaveOutlined } from "@icons";
import { useState } from "react";

import { apiClient } from "../../apiClient";
import { useListQuery } from "../../hooks/useQueries";
import { extractApiError } from "../../apiErrors";
import { humanBytes as formatBytes } from "../../utils/bytes";
import { RestoreDrawer } from "./RestoreDrawer";

type MyBackup = {
  id: string;
  status: string;
  bytes_total: number;
  bytes_added: number;
  created_at: string;
};

const statusColor = (status: string): string => {
  switch (status) {
    case "succeeded":
      return "green";
    case "running":
      return "blue";
    case "failed":
      return "red";
    case "partial":
      return "gold";
    default:
      return "default";
  }
};

export const MyProfileBackupCard = () => {
  const screens = Grid.useBreakpoint();
  const [submitting, setSubmitting] = useState(false);
  const [restoreId, setRestoreId] = useState<string | null>(null);
  const [content, setContent] = useState<string>("full");
  const [compression, setCompression] = useState<string>("");
  const query = useListQuery<MyBackup>({ resource: "me/backups" });

  const handleCreate = async () => {
    setSubmitting(true);
    try {
      await apiClient.post("/me/backups", { content, compression });
      message.success("Backup queued");
      query.refetch();
    } catch (err) {
      message.error(err instanceof Error ? err.message : "Create failed");
    } finally {
      setSubmitting(false);
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await apiClient.delete(`/me/backups/${id}`);
      message.success("Backup deleted");
      query.refetch();
    } catch (err) {
      message.error(extractApiError(err, "Delete failed"));
    }
  };

  const backupActions = (
        <Space wrap>
          <Select
            value={content}
            onChange={setContent}
            style={{ minWidth: 150 }}
            options={[
              { value: "full", label: "Full account" },
              { value: "files", label: "Files only" },
              { value: "database", label: "Databases only" },
            ]}
          />
          <Select
            value={compression}
            onChange={setCompression}
            style={{ minWidth: 140 }}
            options={[
              { value: "", label: "Auto compression" },
              { value: "max", label: "Max compression" },
              { value: "off", label: "No compression" },
            ]}
          />
          <Button type="primary" loading={submitting} onClick={handleCreate}>
            Generate backup
          </Button>
        </Space>
  );

  return (
    <Card
      title={
        <span>
          <SaveOutlined style={{ marginRight: 8 }} />
          My backups
        </span>
      }
      extra={screens.md ? backupActions : undefined}
    >
      {!screens.md ? (
        <div style={{ width: "100%", marginBottom: 12 }}>{backupActions}</div>
      ) : null}
      <Typography.Paragraph type="secondary" style={{ marginTop: 0 }}>
        A full backup bundles your home directory, databases, and mailboxes
        into a portable tar.zst you can download. Backups are deduplicated
        on the server, so repeat runs only store what actually changed.
      </Typography.Paragraph>
      <Table<MyBackup>
        rowKey="id"
        loading={query.isLoading}
        dataSource={query.items ?? []}
        pagination={{ pageSize: 10 }}
        scroll={{ x: "max-content" }}
        columns={[
          {
            title: "Created",
            dataIndex: "created_at",
            render: (t: string) => shortDateTime(t),
          },
          {
            title: "Status",
            dataIndex: "status",
            render: (s: string) => <Tag color={statusColor(s)}>{s}</Tag>,
          },
          {
            title: "Size",
            dataIndex: "bytes_total",
            render: (n: number) => formatBytes(n),
          },
          {
            title: "Actions",
            key: "actions",
            render: (_, row) => (
              <RowActions
                actions={[
                  ...(row.status === "succeeded"
                    ? [
                        { key: "download", label: "Download", icon: <DownloadOutlined />, onClick: () => { const a = getActAs(); window.location.href = `/api/v1/me/backups/${row.id}/download${a ? `?act_as=${encodeURIComponent(a.id)}` : ""}`; } },
                        { key: "restore", label: "Restore", icon: <ReloadOutlined />, onClick: () => setRestoreId(row.id) },
                      ]
                    : []),
                  {
                    key: "delete",
                    label: "Delete",
                    icon: <DeleteOutlined />,
                    danger: true,
                    onClick: () => handleDelete(row.id),
                    confirm: { title: "Delete this backup?", description: "This permanently removes the backup's snapshots from the repository and cannot be undone.", okText: "Delete" },
                  },
                ]}
              />
            ),
          },
        ]}
      />
      <RestoreDrawer
        backupId={restoreId}
        open={restoreId !== null}
        onClose={() => setRestoreId(null)}
      />
    </Card>
  );
};
