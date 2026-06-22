// MyProfileUsageCard — M18 live resource usage for the signed-in user, shown as
// a row of small metric cards. Hits GET /api/v1/users/:id/usage which returns
// { effective, current }. effective is the resolved limits (always present),
// current is the agent's live report (may be absent). Polls every 10s.
import { Card, Col, Progress, Row, Typography } from "antd";
import type { ReactNode } from "react";
import { useEffect, useState } from "react";

import {
  HddOutlined,
  DatabaseOutlined,
  ThunderboltOutlined,
  AppstoreLayoutOutlined,
  DownloadOutlined,
  UploadOutlined,
} from "@icons";

import { apiClient } from "../../apiClient";
import { humanBytes } from "../../utils/bytes";

type Effective = {
  DiskQuotaMB: number;
  CPUQuotaPercent: number;
  MemoryLimitMB: number;
  IOReadMbps: number;
  IOWriteMbps: number;
  MaxTasks: number;
};

type Current = {
  disk?: { used_kb: number; limit_kb: number };
  memory?: { current_bytes: number; max_bytes: number };
  cpu?: { usage_nsec: number; quota_percent: number };
  tasks?: { current: number; max: number };
  io?: { read_bytes: number; write_bytes: number };
};

type UsageResponse = {
  user_id: string;
  effective: Effective;
  current?: Current;
};

function MetricCard({
  icon,
  color,
  label,
  value,
  pct,
}: {
  icon: ReactNode;
  color: string;
  label: string;
  value: string;
  pct?: number;
}) {
  return (
    <Card size="small" styles={{ body: { padding: 12 } }}>
      <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
        <div
          style={{
            flex: "0 0 auto",
            width: 36,
            height: 36,
            borderRadius: 10,
            background: `${color}22`,
            color,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            fontSize: 16,
          }}
        >
          {icon}
        </div>
        <div style={{ minWidth: 0 }}>
          <div style={{ color, fontSize: 12, fontWeight: 600 }}>{label}</div>
          <div style={{ fontSize: 15, fontWeight: 700, lineHeight: 1.2 }}>{value}</div>
        </div>
      </div>
      {pct != null && (
        <Progress
          percent={pct}
          status={pct >= 95 ? "exception" : "active"}
          showInfo={false}
          size="small"
          style={{ marginTop: 8, marginBottom: 0 }}
        />
      )}
    </Card>
  );
}

export function MyProfileUsageCard({ userId }: { userId: string }) {
  const [data, setData] = useState<UsageResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let alive = true;
    const fetch = async () => {
      try {
        const resp = await apiClient.get<UsageResponse>(`/users/${userId}/usage`);
        if (alive) {
          setData(resp.data);
          setError(null);
        }
      } catch (err) {
        if (alive) {
          setError(
            (err as { response?: { data?: { error?: string } } }).response?.data?.error ?? "unavailable",
          );
        }
      }
    };
    fetch();
    const id = setInterval(fetch, 10_000);
    return () => {
      alive = false;
      clearInterval(id);
    };
  }, [userId]);

  const heading = (
    <Typography.Title level={5} style={{ margin: "0 0 12px" }}>
      Resource usage
    </Typography.Title>
  );

  if (error || !data) {
    return (
      <div>
        {heading}
        <Typography.Text type="secondary">
          {error ? "Usage data is currently unavailable." : "Loading…"}
        </Typography.Text>
      </div>
    );
  }

  const { effective, current } = data;

  const diskUsed = (current?.disk?.used_kb ?? 0) * 1024;
  const diskLimitKB =
    (current?.disk?.limit_kb ?? 0) > 0 ? (current?.disk?.limit_kb ?? 0) : effective.DiskQuotaMB * 1024;
  const diskLimit = diskLimitKB * 1024;
  const memUsed = current?.memory?.current_bytes ?? 0;
  const memLimit = current?.memory?.max_bytes ?? effective.MemoryLimitMB * 1024 * 1024;

  const pctOf = (used: number, limit: number) =>
    limit > 0 ? Math.min(100, Math.round((used / limit) * 100)) : undefined;
  const usedOf = (used: number, limit: number) =>
    limit > 0 ? `${humanBytes(used)} / ${humanBytes(limit)}` : `${humanBytes(used)} · ∞`;

  const cpuValue =
    effective.CPUQuotaPercent > 0
      ? `${(effective.CPUQuotaPercent / 100).toFixed(1)} cores`
      : "Unlimited";
  const procValue = current?.tasks
    ? `${current.tasks.current}${current.tasks.max ? ` / ${current.tasks.max}` : ""}`
    : effective.MaxTasks > 0
      ? `Limit ${effective.MaxTasks}`
      : "Unlimited";
  const ioReadValue = effective.IOReadMbps > 0 ? `${effective.IOReadMbps} MB/s` : "Unlimited";
  const ioWriteValue = effective.IOWriteMbps > 0 ? `${effective.IOWriteMbps} MB/s` : "Unlimited";

  const metrics = [
    { icon: <HddOutlined />, color: "#1677ff", label: "Disk", value: usedOf(diskUsed, diskLimit), pct: pctOf(diskUsed, diskLimit) },
    { icon: <DatabaseOutlined />, color: "#9254de", label: "Memory", value: usedOf(memUsed, memLimit), pct: pctOf(memUsed, memLimit) },
    { icon: <ThunderboltOutlined />, color: "#52c41a", label: "CPU quota", value: cpuValue },
    { icon: <AppstoreLayoutOutlined />, color: "#fa8c16", label: "Processes", value: procValue },
    { icon: <DownloadOutlined />, color: "#13c2c2", label: "I/O read", value: ioReadValue },
    { icon: <UploadOutlined />, color: "#eb2f96", label: "I/O write", value: ioWriteValue },
  ];

  return (
    <div>
      {heading}
      <Row gutter={[12, 12]}>
        {metrics.map((m) => (
          <Col xs={12} sm={8} lg={4} key={m.label}>
            <MetricCard {...m} />
          </Col>
        ))}
      </Row>
    </div>
  );
}
