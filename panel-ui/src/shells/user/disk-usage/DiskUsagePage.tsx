// DiskUsagePage — cPanel-style per-user disk-usage breakdown. Shows total
// usage (against the home quota when set) and a per-area breakdown: Files
// (home directory), Email (mailboxes), and Databases. Backed by
// GET /api/v1/me/disk-usage.
import { Alert, Card, Col, Empty, Progress, Row, Spin, Statistic, Table, Tag, Typography } from "antd";
import type { ColumnsType } from "antd/es/table";
import { useQuery } from "@tanstack/react-query";

import { DatabaseOutlined, FolderOutlined, HddOutlined, MailOutlined } from "@icons";

import { apiClient } from "../../../apiClient";

interface UsageItem {
  name: string;
  bytes: number;
  engine?: string;
}
interface UsageCategory {
  bytes: number;
  items: UsageItem[];
}
interface DiskUsage {
  total_bytes: number;
  quota_bytes: number;
  files: UsageCategory;
  email: UsageCategory;
  databases: UsageCategory;
}

function fmtBytes(n: number): string {
  if (!n || n < 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let i = 0;
  let v = n;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v.toFixed(v >= 100 || i === 0 ? 0 : 1)} ${units[i]}`;
}

const CAT_COLORS = { files: "#1677ff", email: "#52c41a", databases: "#faad14" } as const;

export function DiskUsagePage() {
  const { data, isLoading, error } = useQuery<DiskUsage>({
    queryKey: ["me-disk-usage"],
    queryFn: async () => {
      const r = await apiClient.get<DiskUsage>("/me/disk-usage");
      return r.data;
    },
    refetchOnWindowFocus: false,
  });

  if (isLoading) {
    return (
      <div style={{ padding: 48, textAlign: "center" }}>
        <Spin />
      </div>
    );
  }
  if (error || !data) {
    return (
      <Alert
        type="error"
        showIcon
        message="Could not load disk usage"
        description={error instanceof Error ? error.message : "Please try again."}
      />
    );
  }

  const total = data.total_bytes;
  const quota = data.quota_bytes;
  const pct = quota > 0 ? Math.min(100, Math.round((total / quota) * 100)) : 0;

  const categories = [
    { key: "files", label: "Files", icon: <FolderOutlined />, cat: data.files, color: CAT_COLORS.files },
    { key: "email", label: "Email", icon: <MailOutlined />, cat: data.email, color: CAT_COLORS.email },
    {
      key: "databases",
      label: "Databases",
      icon: <DatabaseOutlined />,
      cat: data.databases,
      color: CAT_COLORS.databases,
    },
  ];

  const itemCols: ColumnsType<UsageItem> = [
    { title: "Name", dataIndex: "name", key: "name", ellipsis: true },
    {
      title: "Engine",
      dataIndex: "engine",
      key: "engine",
      width: 110,
      render: (e?: string) => (e ? <Tag>{e}</Tag> : "—"),
    },
    {
      title: "Size",
      dataIndex: "bytes",
      key: "bytes",
      width: 120,
      align: "right",
      sorter: (a, b) => a.bytes - b.bytes,
      defaultSortOrder: "descend",
      render: (b: number) => fmtBytes(b),
    },
  ];

  return (
    <div>
      <Typography.Title level={3} style={{ marginTop: 0 }}>
        <HddOutlined /> Disk Usage
      </Typography.Title>

      {/* Summary */}
      <Card style={{ marginBottom: 16 }}>
        <Row gutter={[24, 16]} align="middle">
          <Col xs={24} md={8}>
            <Statistic title="Total used" value={fmtBytes(total)} />
            <Typography.Text type="secondary">
              {quota > 0 ? `of ${fmtBytes(quota)} quota` : "no home quota set"}
            </Typography.Text>
          </Col>
          <Col xs={24} md={16}>
            {quota > 0 ? (
              <Progress
                percent={pct}
                status={pct >= 90 ? "exception" : "active"}
                format={(p) => `${p}%`}
              />
            ) : (
              <Typography.Text type="secondary">Unlimited / quota not enforced.</Typography.Text>
            )}
            {/* Proportional breakdown bar */}
            {total > 0 && (
              <div style={{ display: "flex", height: 14, borderRadius: 6, overflow: "hidden", marginTop: 12 }}>
                {categories.map((c) =>
                  c.cat.bytes > 0 ? (
                    <div
                      key={c.key}
                      title={`${c.label}: ${fmtBytes(c.cat.bytes)}`}
                      style={{ width: `${(c.cat.bytes / total) * 100}%`, background: c.color }}
                    />
                  ) : null,
                )}
              </div>
            )}
          </Col>
        </Row>
      </Card>

      {/* Category cards */}
      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        {categories.map((c) => (
          <Col xs={24} sm={8} key={c.key}>
            <Card size="small">
              <Statistic
                title={
                  <span style={{ color: c.color }}>
                    {c.icon} {c.label}
                  </span>
                }
                value={fmtBytes(c.cat.bytes)}
              />
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                {c.cat.items.length} item{c.cat.items.length === 1 ? "" : "s"}
                {total > 0 ? ` · ${Math.round((c.cat.bytes / total) * 100)}%` : ""}
              </Typography.Text>
            </Card>
          </Col>
        ))}
      </Row>

      {/* Breakdown tables */}
      {categories.map((c) => (
        <Card
          key={c.key}
          size="small"
          title={
            <span>
              {c.icon} {c.label} — {fmtBytes(c.cat.bytes)}
            </span>
          }
          style={{ marginBottom: 16 }}
        >
          {c.cat.items.length === 0 ? (
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={`No ${c.label.toLowerCase()}`} />
          ) : (
            <Table<UsageItem>
              size="small"
              rowKey={(r) => `${c.key}:${r.name}`}
              columns={c.key === "databases" ? itemCols : itemCols.filter((col) => col.key !== "engine")}
              dataSource={c.cat.items}
              pagination={c.cat.items.length > 10 ? { pageSize: 10, size: "small" } : false}
              scroll={{ x: "max-content" }}
            />
          )}
        </Card>
      ))}

      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
        Email usage is sampled periodically. PostgreSQL database sizes are not yet reported.
      </Typography.Text>
    </div>
  );
}
