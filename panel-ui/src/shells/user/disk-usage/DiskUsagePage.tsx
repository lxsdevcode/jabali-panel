// DiskUsagePage — cPanel-style per-user disk-usage breakdown. Top row: five
// stat cards (Total / Files / Email / Databases / Quota). Bottom row: three
// breakdown cards (Files & Folders, Email Mailboxes, Databases) each with a
// table + a "View all" link. Backed by GET /api/v1/me/disk-usage.
import { Alert, Card, Col, Empty, Row, Spin, Table, Tag, Tree, Typography } from "antd";
import type { ColumnsType } from "antd/es/table";
import type { DataNode } from "antd/es/tree";
import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "react-router";

import { DatabaseOutlined, ExportOutlined, FileOutlined, FolderOutlined, MailOutlined } from "@icons";

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

interface DuEntry {
  name: string;
  is_dir: boolean;
  size: number;
  has_subdirs: boolean;
}
interface DuResp {
  path: string;
  total: number;
  entries: DuEntry[];
}

function nodeTitle(isDir: boolean, name: string, size: number) {
  return (
    <span style={{ display: "flex", alignItems: "center", gap: 8, width: "100%" }}>
      <span style={{ flex: "0 0 auto", display: "inline-flex", color: isDir ? "#1677ff" : undefined }}>
        {isDir ? <FolderOutlined /> : <FileOutlined />}
      </span>
      <span style={{ flex: 1, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
        {name}
      </span>
      <Typography.Text type="secondary" style={{ fontSize: 12, flex: "0 0 auto" }}>
        {fmtBytes(size)}
      </Typography.Text>
    </span>
  );
}

function toNodes(entries: DuEntry[], parentPath: string): DataNode[] {
  return entries.map((e) => ({
    key: `${parentPath}/${e.name}`,
    title: nodeTitle(e.is_dir, e.name, e.size),
    isLeaf: !e.is_dir,
  }));
}

function updateTreeData(list: DataNode[], key: string, children: DataNode[]): DataNode[] {
  return list.map((node) => {
    if (node.key === key) return { ...node, children };
    if (node.children) return { ...node, children: updateTreeData(node.children, key, children) };
    return node;
  });
}

// FilesTree — lazy disk-usage tree of the user's home directory. Each folder
// carries its recursive size; expanding a folder fetches the next level on
// demand (GET /me/disk-usage/files?path=).
function FilesTree() {
  const [treeData, setTreeData] = useState<DataNode[]>([]);
  const [loading, setLoading] = useState(true);

  const fetchDir = async (path?: string): Promise<DuResp> => {
    const url = path ? `/me/disk-usage/files?path=${encodeURIComponent(path)}` : "/me/disk-usage/files";
    const r = await apiClient.get<DuResp>(url);
    return r.data;
  };

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const d = await fetchDir();
        if (!cancelled) setTreeData(toNodes(d.entries, d.path));
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const onLoadData = async (node: DataNode) => {
    const key = node.key as string;
    const d = await fetchDir(key);
    setTreeData((prev) => updateTreeData(prev, key, toNodes(d.entries, key)));
  };

  if (loading) {
    return (
      <div style={{ padding: 24, textAlign: "center" }}>
        <Spin />
      </div>
    );
  }
  if (treeData.length === 0) {
    return (
      <div style={{ padding: 24 }}>
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="Home directory is empty" />
      </div>
    );
  }
  return (
    <div style={{ padding: "8px 12px", maxHeight: 320, overflow: "auto" }}>
      <Tree blockNode treeData={treeData} loadData={onLoadData} />
    </div>
  );
}

const COLORS = {
  purple: "#722ed1",
  blue: "#1677ff",
  green: "#52c41a",
  gold: "#faad14",
} as const;

function StatCard({
  label,
  value,
  sub,
  color,
  icon,
}: {
  label: string;
  value: string;
  sub: string;
  color: string;
  icon: React.ReactNode;
}) {
  return (
    <Card size="small" styles={{ body: { padding: 16 } }}>
      <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
        <div
          style={{
            flex: "0 0 auto",
            width: 48,
            height: 48,
            borderRadius: 12,
            background: `${color}22`,
            color,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            fontSize: 22,
          }}
        >
          {icon}
        </div>
        <div style={{ minWidth: 0 }}>
          <div style={{ color, fontSize: 13, fontWeight: 600, marginBottom: 2 }}>{label}</div>
          <div style={{ fontSize: 26, fontWeight: 700, lineHeight: 1.1 }}>{value}</div>
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            {sub}
          </Typography.Text>
        </div>
      </div>
    </Card>
  );
}

export function DiskUsagePage() {
  const navigate = useNavigate();
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
  const pctOf = (n: number) => (total > 0 ? `${Math.round((n / total) * 100)}% of total` : "0% of total");
  const quotaPct = quota > 0 ? Math.min(100, Math.round((total / quota) * 100)) : 0;

  const statCards = [
    {
      label: "Total Used",
      value: fmtBytes(total),
      sub: "Across all data",
      color: COLORS.purple,
      icon: <DatabaseOutlined />,
    },
    {
      label: "Files",
      value: fmtBytes(data.files.bytes),
      sub: pctOf(data.files.bytes),
      color: COLORS.blue,
      icon: <FileOutlined />,
    },
    {
      label: "Email",
      value: fmtBytes(data.email.bytes),
      sub: pctOf(data.email.bytes),
      color: COLORS.green,
      icon: <MailOutlined />,
    },
    {
      label: "Databases",
      value: fmtBytes(data.databases.bytes),
      sub: pctOf(data.databases.bytes),
      color: COLORS.gold,
      icon: <DatabaseOutlined />,
    },
    {
      label: "Quota Status",
      value: quota > 0 ? `${quotaPct}%` : "Unlimited",
      sub: quota > 0 ? `of ${fmtBytes(quota)}` : "No quota enforced",
      color: COLORS.purple,
      icon: <span style={{ fontSize: 24, fontWeight: 700, lineHeight: 1 }}>∞</span>,
    },
  ];

  const sizeCol = {
    title: "Size",
    dataIndex: "bytes",
    key: "bytes",
    width: 110,
    align: "right" as const,
    sorter: (a: UsageItem, b: UsageItem) => a.bytes - b.bytes,
    defaultSortOrder: "descend" as const,
    render: (b: number) => fmtBytes(b),
  };

  const filesCols: ColumnsType<UsageItem> = [
    { title: "Name", dataIndex: "name", key: "name", ellipsis: true },
    sizeCol,
  ];
  const emailCols: ColumnsType<UsageItem> = [
    { title: "Mailbox", dataIndex: "name", key: "name", ellipsis: true },
    sizeCol,
  ];
  const dbCols: ColumnsType<UsageItem> = [
    { title: "Database", dataIndex: "name", key: "name", ellipsis: true },
    {
      title: "Engine",
      dataIndex: "engine",
      key: "engine",
      width: 100,
      render: (e?: string) => e || "—",
    },
    sizeCol,
  ];

  const breakdowns = [
    {
      key: "files",
      title: "Files & Folders",
      icon: <FileOutlined />,
      color: COLORS.blue,
      cat: data.files,
      cols: filesCols,
      emptyText: "No files",
      viewAll: { label: "View all files", to: "/jabali-panel/files" },
    },
    {
      key: "email",
      title: "Email Mailboxes",
      icon: <MailOutlined />,
      color: COLORS.green,
      cat: data.email,
      cols: emailCols,
      emptyText: "No email usage yet",
      emptyHint: "Your mailboxes are currently empty.",
      viewAll: { label: "View all mailboxes", to: "/jabali-panel/mail/mailboxes" },
    },
    {
      key: "databases",
      title: "Databases",
      icon: <DatabaseOutlined />,
      color: COLORS.gold,
      cat: data.databases,
      cols: dbCols,
      emptyText: "No databases",
      viewAll: { label: "View all databases", to: "/jabali-panel/databases" },
    },
  ];

  return (
    <div>
      <Typography.Title level={3} style={{ marginTop: 0 }}>
        Disk Usage
      </Typography.Title>

      {/* Top: five stat cards */}
      <div style={{ display: "flex", flexWrap: "wrap", gap: 12, marginBottom: 16 }}>
        {statCards.map((c) => (
          <div key={c.label} style={{ flex: "1 1 200px", minWidth: 180 }}>
            <StatCard {...c} />
          </div>
        ))}
      </div>

      {/* Storage quota usage strip */}
      <Card size="small" style={{ marginBottom: 16 }} styles={{ body: { padding: 16 } }}>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "baseline", marginBottom: 10 }}>
          <span>
            <Typography.Text strong>Storage Quota Usage</Typography.Text>{" "}
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              {quota > 0 ? `${quotaPct}% used` : "No quota enforced"}
            </Typography.Text>
          </span>
          <span>
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              {fmtBytes(total)} used
            </Typography.Text>{" "}
            <Typography.Text strong>{quota > 0 ? fmtBytes(quota) : "Unlimited"}</Typography.Text>
          </span>
        </div>
        <div
          style={{
            height: 14,
            borderRadius: 8,
            background: "rgba(128,128,128,0.18)",
            overflow: "hidden",
          }}
        >
          <div
            style={{
              height: "100%",
              width: quota > 0 ? `${quotaPct}%` : "100%",
              borderRadius: 8,
              background:
                "repeating-linear-gradient(45deg, #1677ff, #1677ff 10px, #4096ff 10px, #4096ff 20px)",
            }}
          />
        </div>
        <Typography.Text type="secondary" style={{ fontSize: 12, display: "block", marginTop: 8 }}>
          {quota > 0
            ? `${fmtBytes(total)} of ${fmtBytes(quota)} (${quotaPct}%)`
            : "Unlimited / no quota enforced"}
        </Typography.Text>
      </Card>

      {/* Bottom: three breakdown cards */}
      <Row gutter={[16, 16]}>
        {breakdowns.map((b) => (
          <Col xs={24} lg={8} key={b.key}>
            <Card
              size="small"
              title={
                <span style={{ color: b.color }}>
                  {b.icon} <span style={{ color: "inherit" }}>{b.title}</span>
                </span>
              }
              extra={
                <Tag
                  style={{
                    color: b.color,
                    background: `${b.color}22`,
                    borderColor: `${b.color}55`,
                    fontWeight: 600,
                  }}
                >
                  {fmtBytes(b.cat.bytes)}
                </Tag>
              }
              styles={{ body: { padding: 0, minHeight: 220, display: "flex", flexDirection: "column" } }}
            >
              <div style={{ flex: 1 }}>
                {b.key === "files" ? (
                  <FilesTree />
                ) : b.cat.items.length === 0 ? (
                  <div style={{ padding: 24 }}>
                    <Empty
                      image={Empty.PRESENTED_IMAGE_SIMPLE}
                      description={
                        <div>
                          <div>{b.emptyText}</div>
                          {b.emptyHint && (
                            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                              {b.emptyHint}
                            </Typography.Text>
                          )}
                        </div>
                      }
                    />
                  </div>
                ) : (
                  <Table<UsageItem>
                    size="small"
                    rowKey={(r) => `${b.key}:${r.name}`}
                    columns={b.cols}
                    dataSource={b.cat.items}
                    pagination={b.cat.items.length > 8 ? { pageSize: 8, size: "small" } : false}
                    scroll={{ x: "max-content" }}
                  />
                )}
              </div>
              <div style={{ padding: "10px 16px", borderTop: "1px solid rgba(128,128,128,0.15)" }}>
                <Typography.Link onClick={() => navigate(b.viewAll.to)}>
                  {b.viewAll.label} <ExportOutlined />
                </Typography.Link>
              </div>
            </Card>
          </Col>
        ))}
      </Row>

      <Typography.Text type="secondary" style={{ fontSize: 12, display: "block", marginTop: 12 }}>
        Email usage is sampled periodically. PostgreSQL database sizes are not yet reported.
      </Typography.Text>
    </div>
  );
}
