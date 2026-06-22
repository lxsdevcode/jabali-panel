// RestoreDrawer — GH #267 Wave 4. Tenant self-service restore UI.
//
// DB-ONLY in v1 (matches the backend): preview the backup's databases, pick a
// subset, explicitly confirm the destructive overwrite, then POST the restore.
// Home / mail / DNS are intentionally not offered — see
// plans/m267-tenant-selective-restore.md (mail is RocksDB-unsafe via file
// apply; home rsync uses --delete; custom DNS records aren't captured).
import { useEffect, useState } from "react";

import {
  Alert,
  Button,
  Checkbox,
  Drawer,
  Empty,
  Space,
  Spin,
  Typography,
  message,
} from "antd";

import { apiClient } from "../../apiClient";

interface ManifestStage {
  name: string;
  status: string;
  items?: string[];
}
interface ManifestResponse {
  kind: string;
  username: string;
  stages: ManifestStage[];
}
interface RestoreResult {
  applied?: string[] | null;
  skipped?: string[] | null;
  warnings?: string[] | null;
}

interface RestoreDrawerProps {
  backupId: string | null;
  open: boolean;
  onClose: () => void;
}

export const RestoreDrawer = ({ backupId, open, onClose }: RestoreDrawerProps) => {
  const [loading, setLoading] = useState(false);
  const [databases, setDatabases] = useState<string[]>([]);
  const [selected, setSelected] = useState<string[]>([]);
  const [confirmOverwrite, setConfirmOverwrite] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [result, setResult] = useState<RestoreResult | null>(null);

  useEffect(() => {
    if (!open || !backupId) return;
    setLoading(true);
    setDatabases([]);
    setSelected([]);
    setConfirmOverwrite(false);
    setResult(null);
    apiClient
      .get<ManifestResponse>(`/me/backups/${backupId}/manifest`)
      .then((resp) => {
        const dbs = (resp.data.stages ?? [])
          .filter((s) => s.name === "db" && s.status === "ok")
          .flatMap((s) => s.items ?? []);
        setDatabases(dbs);
      })
      .catch((err) =>
        message.error(
          err instanceof Error ? err.message : "Could not read backup contents",
        ),
      )
      .finally(() => setLoading(false));
  }, [open, backupId]);

  const handleRestore = async () => {
    if (!backupId || selected.length === 0) return;
    setSubmitting(true);
    setResult(null);
    try {
      const resp = await apiClient.post<RestoreResult>(
        `/me/backups/${backupId}/restore`,
        { databases: selected, overwrite: true },
      );
      setResult(resp.data);
      const n = resp.data.applied?.length ?? 0;
      if (n > 0) message.success(`Restored ${n} database${n === 1 ? "" : "s"}`);
      else message.warning("Nothing was restored — see details");
    } catch (err) {
      message.error(err instanceof Error ? err.message : "Restore failed");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Drawer
      title="Restore from backup"
      width={460}
      open={open}
      onClose={onClose}
      destroyOnClose
    >
      {loading ? (
        <div style={{ textAlign: "center", padding: 48 }}>
          <Spin />
        </div>
      ) : (
        <Space direction="vertical" size="middle" style={{ width: "100%" }}>
          <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
            Select the databases to restore from this backup. Restoring a
            database <strong>replaces its current contents</strong> with the
            backed-up copy — anything created since the backup is lost. Other
            databases, your files, and mail are not touched.
          </Typography.Paragraph>

          {databases.length === 0 ? (
            <Empty
              image={Empty.PRESENTED_IMAGE_SIMPLE}
              description="This backup contains no databases"
            />
          ) : (
            <>
              <Checkbox.Group
                value={selected}
                onChange={(v) => setSelected(v as string[])}
                style={{ display: "flex", flexDirection: "column", gap: 8 }}
                options={databases.map((d) => ({ label: d, value: d }))}
              />

              {selected.length > 0 && (
                <Alert
                  type="warning"
                  showIcon
                  message="This is destructive"
                  description={`The selected database${
                    selected.length === 1 ? "" : "s"
                  } will be dropped and reloaded from the backup.`}
                />
              )}

              <Checkbox
                checked={confirmOverwrite}
                onChange={(e) => setConfirmOverwrite(e.target.checked)}
              >
                I understand this replaces the selected database(s).
              </Checkbox>

              <Button
                type="primary"
                danger
                loading={submitting}
                disabled={selected.length === 0 || !confirmOverwrite}
                onClick={handleRestore}
              >
                Restore {selected.length || ""} database
                {selected.length === 1 ? "" : "s"}
              </Button>
            </>
          )}

          {result && (
            <Alert
              type={(result.applied?.length ?? 0) > 0 ? "success" : "info"}
              showIcon
              message="Restore result"
              description={
                <Space direction="vertical" size={2}>
                  {(result.applied ?? []).map((a) => (
                    <span key={a}>✓ {a}</span>
                  ))}
                  {(result.skipped ?? []).map((s) => (
                    <span key={s}>• skipped: {s}</span>
                  ))}
                  {(result.warnings ?? []).map((w, i) => (
                    <Typography.Text type="secondary" key={i}>
                      {w}
                    </Typography.Text>
                  ))}
                </Space>
              }
            />
          )}
        </Space>
      )}
    </Drawer>
  );
};
