// AdminUserOverview — per-user hub for admin cross-entity navigation (#483,
// ADR-0152, Wave C). The one new admin detail route. It is the breadcrumb root
// for drilling into a user and the target of every "→ owner" link. Resource
// cards link to the owner-scoped admin lists (?user_id=) and show live counts
// read from each list's `total` (page_size=1) — no dedicated counts endpoint.
import {
  AppstoreOutlined,
  CloudServerOutlined,
  ContainerOutlined,
  GlobalOutlined,
  MailOutlined,
  UserOutlined,
} from "@icons";
import { useQuery } from "@tanstack/react-query";
import { Alert, Card, Col, Row, Skeleton, Statistic, Tag, Typography } from "antd";
import { Link, useParams } from "react-router";

import { apiClient } from "../../../apiClient";
import { AdminBreadcrumb } from "../../../components/admin/AdminBreadcrumb";
import { adminLinks, ownerCrumbs, ownerLabel } from "../../../components/admin/entityLinks";

type AdminUser = {
  id: string;
  email: string;
  username?: string | null;
  name_first: string;
  name_last: string;
  is_admin: boolean;
  package_id?: string | null;
};

// Each count comes from the owner-scoped list endpoint. Domains/backups
// paginate and expose `total`; the admin mailbox + docker endpoints return the
// whole owner set, so fall back to the array length.
async function countDomains(id: string): Promise<number> {
  const { data } = await apiClient.get("/domains", { params: { user_id: id, page_size: 1 } });
  return data.total ?? 0;
}
async function countMailboxes(id: string): Promise<number> {
  const { data } = await apiClient.get("/admin/mailboxes", { params: { user_id: id } });
  return data.total ?? data.data?.length ?? 0;
}
async function countDockerApps(id: string): Promise<number> {
  const { data } = await apiClient.get("/admin/docker-apps", { params: { user_id: id } });
  return data.items?.length ?? 0;
}
async function countBackups(id: string): Promise<number> {
  const { data } = await apiClient.get("/admin/backups", { params: { user_id: id, page_size: 1 } });
  return data.total ?? 0;
}

type ResourceCard = {
  key: string;
  label: string;
  icon: React.ReactNode;
  href: string;
  count?: number;
};

export function AdminUserOverview() {
  const { id = "" } = useParams();

  const userQ = useQuery({
    queryKey: ["admin-user", id],
    queryFn: async () => (await apiClient.get<AdminUser>(`/users/${id}`)).data,
    enabled: id !== "",
  });

  const countsQ = useQuery({
    queryKey: ["admin-user-resource-counts", id],
    enabled: id !== "",
    queryFn: async () => {
      const [domains, mailboxes, dockerApps, backups] = await Promise.all([
        countDomains(id).catch(() => undefined),
        countMailboxes(id).catch(() => undefined),
        countDockerApps(id).catch(() => undefined),
        countBackups(id).catch(() => undefined),
      ]);
      return { domains, mailboxes, dockerApps, backups };
    },
  });

  if (userQ.isLoading) {
    return <Skeleton active paragraph={{ rows: 4 }} />;
  }
  if (userQ.isError || !userQ.data) {
    return <Alert type="error" showIcon message="User not found" />;
  }

  const user = userQ.data;
  const counts = countsQ.data;

  const cards: ResourceCard[] = [
    { key: "domains", label: "Domains", icon: <GlobalOutlined />, href: adminLinks.domains(id), count: counts?.domains },
    { key: "mailboxes", label: "Mailboxes", icon: <MailOutlined />, href: adminLinks.mailboxes(id), count: counts?.mailboxes },
    { key: "docker-apps", label: "Docker Apps", icon: <AppstoreOutlined />, href: adminLinks.dockerApps(id), count: counts?.dockerApps },
    { key: "backups", label: "Backups", icon: <CloudServerOutlined />, href: adminLinks.backups(id), count: counts?.backups },
  ];

  const fullName = [user.name_first, user.name_last].filter(Boolean).join(" ");

  return (
    <div>
      <AdminBreadcrumb items={ownerCrumbs(user)} />

      <Typography.Title level={3} style={{ marginTop: 0, marginBottom: 4 }}>
        <UserOutlined /> {ownerLabel(user)}
        {user.is_admin && (
          <Tag color="gold" style={{ marginInlineStart: 8 }}>
            Admin
          </Tag>
        )}
      </Typography.Title>
      <Typography.Paragraph type="secondary" style={{ marginBottom: 20 }}>
        {fullName ? `${fullName} · ` : ""}
        {user.email}
      </Typography.Paragraph>

      <Row gutter={[16, 16]}>
        {cards.map((c) => (
          <Col key={c.key} xs={24} sm={12} md={8} lg={6}>
            <Link to={c.href}>
              <Card hoverable size="small">
                <Statistic
                  title={
                    <span>
                      {c.icon} {c.label}
                    </span>
                  }
                  value={c.count ?? "—"}
                  loading={countsQ.isLoading}
                />
              </Card>
            </Link>
          </Col>
        ))}
        {user.package_id && (
          <Col xs={24} sm={12} md={8} lg={6}>
            <Link to={adminLinks.packageEdit(user.package_id)}>
              <Card hoverable size="small">
                <Statistic
                  title={
                    <span>
                      <ContainerOutlined /> Package
                    </span>
                  }
                  value="View"
                />
              </Card>
            </Link>
          </Col>
        )}
      </Row>
    </div>
  );
}
