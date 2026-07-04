// AdminSecurityAide — admin Security tab "AIDE" sub-tab (M42, ADR-0087).
// Read-only FIM status + manual recheck trigger.
import { Alert, Badge, Button, Card, Popconfirm, Space, Statistic, Table, Tag, Tooltip, Typography, message } from "antd";

import {
  type AideSampleRow,
  useAideStatus,
  useRunAideCheck,
  useRunAideRebuild,
} from "../../../hooks/useSecurityAide";

const CATEGORY_COLOR: Record<string, string> = {
  "security-control": "red",
  unknown: "volcano",
  "jabali-managed": "blue",
  certificate: "purple",
  "system-package": "default",
  "log-state": "default",
  "user-content": "green",
};
const CHANGE_COLOR: Record<AideSampleRow["change_type"], string> = {
  added: "green",
  changed: "orange",
  removed: "red",
};

function humanizeAge(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) return "—";
  const d = Math.floor(seconds / 86400);
  const h = Math.floor((seconds % 86400) / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  if (d > 0) return `${d}d ${h}h`;
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m`;
}

export const AdminSecurityAide = () => {
  const { data, isLoading, refetch } = useAideStatus();
  const runCheck = useRunAideCheck();
  const runRebuild = useRunAideRebuild();

  if (isLoading) {
    return (
      <Card title="AIDE — file integrity monitor" size="small">
        <Typography.Text type="secondary">Loading…</Typography.Text>
      </Card>
    );
  }

  if (!data?.enabled) {
    return (
      <Card title="AIDE — file integrity monitor" size="small">
        <Alert
          type="warning"
          showIcon
          message="AIDE not active"
          description={data?.reason || "AIDE not installed or DB missing. install_aide() runs on jabali update."}
        />
      </Card>
    );
  }

  const total = data.summary.added + data.summary.changed + data.summary.removed;

  return (
    <Card
      title="AIDE — file integrity monitor"
      size="small"
      extra={
        <Space>
          <Button size="small" onClick={() => refetch()}>
            Refresh
          </Button>
          <Button
            size="small"
            type="primary"
            loading={runCheck.isPending}
            onClick={() =>
              runCheck.mutate(undefined, {
                onSuccess: () => message.success("Check complete"),
                onError: () => message.error("Check failed — see agent logs"),
              })
            }
          >
            Run check now
          </Button>
          <Button
            size="small"
            loading={runRebuild.isPending}
            onClick={() =>
              runRebuild.mutate(true, {
                onSuccess: (d) => message.info(`Dry run: would run ${d.plan ?? "aideinit -y -f"}`),
                onError: () => message.error("Dry run failed — see agent logs"),
              })
            }
          >
            Rebuild (dry run)
          </Button>
          <Popconfirm
            title="Rebuild the AIDE baseline?"
            description="This replaces the integrity baseline with the current filesystem state (aideinit -y -f). Only do this after deliberate changes — any tampering present now becomes the new 'clean' baseline."
            okText="Rebuild"
            okButtonProps={{ danger: true }}
            onConfirm={() =>
              runRebuild.mutate(false, {
                onSuccess: () => message.success("Baseline rebuilt"),
                onError: () => message.error("Rebuild failed — see agent logs"),
              })
            }
          >
            <Button size="small" danger loading={runRebuild.isPending}>
              Rebuild baseline
            </Button>
          </Popconfirm>
        </Space>
      }
    >
      <Typography.Paragraph type="secondary" style={{ marginTop: 0 }}>
        Daily SHA-256 hash check on system binaries + configs
        (<code>/bin /sbin /usr/bin /usr/sbin /lib /etc /boot /root</code>).
        Excludes paths the panel writes to (
        <code>/etc/jabali/</code>, <code>/etc/letsencrypt/live/</code>,
        reconciler-managed configs). Daily timer
        <code>jabali-aide-check.timer</code> runs at 04:30 UTC + jitter.
      </Typography.Paragraph>

      <Space size="large" style={{ marginBottom: 16 }}>
        <Statistic title="DB age" value={humanizeAge(data.db_age_seconds)} />
        <Statistic title="Last check" value={data.last_check_ts || "—"} />
        <Statistic title="Added" value={data.summary.added} />
        <Statistic title="Changed" value={data.summary.changed} />
        <Statistic title="Removed" value={data.summary.removed} />
      </Space>

      {total > 0 ? (
        <Alert
          type="warning"
          showIcon
          message={`${total} file(s) changed since the last baseline`}
          description="Review the sample below. If the changes are expected (kernel bump, manual config edit), re-baseline via SSH: jabali aide rebuild --full."
          style={{ marginBottom: 12 }}
        />
      ) : (
        <Alert
          type="success"
          showIcon
          message="No tamper detected"
          description="All watched paths match the baseline checksum."
          style={{ marginBottom: 12 }}
        />
      )}

      {data.categories && Object.keys(data.categories).length > 0 && (
        <div style={{ marginBottom: 16 }}>
          <Typography.Text strong>Changes by source (GH #714): </Typography.Text>
          <Space wrap style={{ marginTop: 8 }}>
            {Object.entries(data.categories)
              .sort((a, b) => b[1] - a[1])
              .map(([cat, n]) => (
                <Tag key={cat} color={CATEGORY_COLOR[cat] ?? "default"}>
                  {cat}: {n}
                </Tag>
              ))}
          </Space>
          {(data.categories["unknown"] || data.categories["security-control"]) ? (
            <Alert
              type="warning"
              showIcon
              style={{ marginTop: 12 }}
              message="Prioritize these"
              description="`security-control` and `unknown/unowned` changes deserve scrutiny before re-baselining — the rest is usually package/kernel churn."
            />
          ) : null}
        </div>
      )}
      <Table
        rowKey={(r, idx) => `${r.change_type}-${r.path}-${idx}`}
        dataSource={data.sample}
        size="small"
        tableLayout="fixed"
        scroll={{ x: "max-content" }}
        pagination={{ pageSize: 25 }}
        columns={[
          {
            title: "Change",
            dataIndex: "change_type",
            width: 110,
            render: (v: AideSampleRow["change_type"]) => (
              <Tag color={CHANGE_COLOR[v]}>{v}</Tag>
            ),
          },
          {
            title: "Category",
            dataIndex: "category",
            width: 150,
            render: (v: string) => <Tag color={CATEGORY_COLOR[v] ?? "default"}>{v || "?"}</Tag>,
          },
          {
            title: "Path",
            dataIndex: "path",
            ellipsis: { showTitle: false } as const,
            render: (v: string) => (
              <Tooltip title={v}>
                <code>{v}</code>
              </Tooltip>
            ),
          },
        ]}
      />

      <Typography.Paragraph type="secondary" style={{ marginTop: 16, marginBottom: 0 }}>
        <Badge status="processing" /> Re-baseline after a deliberate change with{" "}
        <code>jabali aide rebuild --full</code>. <code>jabali update</code>{" "}
        re-baselines panel binaries automatically.
      </Typography.Paragraph>
    </Card>
  );
};
