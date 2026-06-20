// MailboxesTab — cross-domain mailbox list + actions.
//
// Lists mailboxes from all user domains in a single table. Extracted from
// the original UserMailboxesPage. Merges results from per-domain list queries
// client-side and provides password rotation, SSO mint, and delete actions.
import { useMemo, useState } from "react";
import {
  Button,
  Dropdown,
  Empty,
  Form,
  Modal,
  Progress,
  Skeleton,
  Space,
  Table,
  Tag,
  Tooltip,
  Typography,
  message,
} from "antd";
import type { MenuProps } from "antd";
import {
  DeleteOutlined,
  EditOutlined,
  ClockCircleOutlined,
  KeyOutlined,
  MailOutlined,
  MoreOutlined,
} from "@icons";
import { LibravatarAvatar } from "../../../../components/LibravatarAvatar";
import { RowActionButton } from "../../../../components/RowActionButton";
import { useQueries } from "@tanstack/react-query";

import { AutoReplyModal } from "../AutoReplyModal";
import { type Autoresponder } from "../../../../hooks/useAutoresponders";

import { apiClient } from "../../../../apiClient";
import {
  useDeleteMailbox,
  useMintMailboxSSO,
  useRotateMailboxPassword,
  type Mailbox,
} from "../../../../hooks/useMailboxes";
import { useListQuery } from "../../../../hooks/useQueries";
import type { Domain } from "../../domains/UserDomainList";
import { DatabaseUserPasswordModal } from "../../../../components/DatabaseUserPasswordModal";
import { PasswordInput } from "../../../../components/PasswordInput";
import { EditMailboxModal } from "../../../../components/mail/EditMailboxModal";

type MailboxRow = Mailbox & { domain_name: string };
type GroupMembership = {
  group_id: string;
  group_name: string;
  group_email: string;
};

function formatBytes(n: number): string {
  if (n === 0) return "0 B";
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  const i = Math.floor(Math.log(n) / Math.log(1024));
  const v = n / Math.pow(1024, i);
  return `${v.toFixed(v >= 10 || i === 0 ? 0 : 1)} ${units[i]}`;
}

export const MailboxesTab = () => {
  const { items: domains, isLoading: loadingDomains } = useListQuery<Domain>({
    resource: "domains",
    params: { page: 1, pageSize: 200, sort: "name", order: "asc" },
  });

  const emailEnabledDomains = useMemo(
    () => domains.filter((d) => d.email_enabled),
    [domains],
  );

  const mailboxResults = useQueries({
    queries: emailEnabledDomains.map((d) => ({
      queryKey: ["list", "mailboxes", d.id, { page: 1, pageSize: 200 }],
      queryFn: async () => {
        const { data } = await apiClient.get<{ data: Mailbox[]; total: number }>(
          `/domains/${d.id}/mailboxes?page=1&page_size=200&sort=local_part&order=asc`,
        );
        return { items: data.data ?? [], domain: d };
      },
    })),
  });

  const membershipResults = useQueries({
    queries: emailEnabledDomains.map((d) => ({
      queryKey: ["list", "mailbox-group-memberships", d.id],
      queryFn: async () => {
        const { data } = await apiClient.get<{
          data: Record<string, GroupMembership[]>;
        }>(`/domains/${d.id}/mailbox-group-memberships`);
        return data.data ?? {};
      },
    })),
  });

  const groupsByMailbox = useMemo(() => {
    const out: Record<string, GroupMembership[]> = {};
    for (const r of membershipResults) {
      if (!r.data) continue;
      for (const [mbID, groups] of Object.entries(r.data)) {
        out[mbID] = groups;
      }
    }
    return out;
  }, [membershipResults]);


  const rows: MailboxRow[] = useMemo(() => {
    const out: MailboxRow[] = [];
    for (const r of mailboxResults) {
      if (!r.data) continue;
      for (const mb of r.data.items) {
        out.push({ ...mb, domain_name: r.data.domain.name });
      }
    }
    return out;
  }, [mailboxResults]);

  // Per-mailbox automatic-replies (autoresponder) fanout — one GET each,
  // shares the cache with useAutoresponder via the matching query key.
  // GH #240: surfaced as a column + kebab action on this tab (the
  // standalone Autoresponders tab was removed).
  const arResults = useQueries({
    queries: emailEnabledDomains.map((d) => ({
      queryKey: ["autoresponders", "by-domain", d.id],
      queryFn: async () => {
        const { data } = await apiClient.get<{
          data: Record<string, Autoresponder>;
        }>(`/domains/${d.id}/autoresponders`);
        return data.data ?? {};
      },
    })),
  });

  const arByMailbox = useMemo(() => {
    const out: Record<string, Autoresponder> = {};
    for (const r of arResults) {
      if (!r.data) continue;
      for (const [mbID, ar] of Object.entries(r.data)) {
        out[mbID] = ar;
      }
    }
    return out;
  }, [arResults]);


  const [passwordModal, setPasswordModal] = useState<{
    email: string;
    password: string;
    title: string;
  } | null>(null);
  const [rotatingId, setRotatingId] = useState<string | null>(null);
  const [resetTarget, setResetTarget] = useState<MailboxRow | null>(null);
  const [editTarget, setEditTarget] = useState<MailboxRow | null>(null);
  const [arTarget, setArTarget] = useState<MailboxRow | null>(null);
  const [resetForm] = Form.useForm<{ password?: string }>();

  const deleteMutation = useDeleteMailbox();
  const rotateMutation = useRotateMailboxPassword();
  const ssoMutation = useMintMailboxSSO();

  const openReset = (row: MailboxRow) => {
    resetForm.resetFields();
    setResetTarget(row);
  };

  const submitReset = async () => {
    if (!resetTarget) return;
    const values = await resetForm.validateFields();
    const custom = (values.password ?? "").trim();
    setRotatingId(resetTarget.id);
    try {
      const resp = await rotateMutation.mutateAsync({
        id: resetTarget.id,
        new_password: custom || undefined,
      });
      setResetTarget(null);
      if (resp.password) {
        // No custom password supplied -> server generated one; reveal once.
        setPasswordModal({
          email: resetTarget.email,
          password: resp.password,
          title: "New mailbox password (auto-generated)",
        });
      } else {
        message.success("Password updated");
      }
    } catch (err) {
      const msg =
        (err as { response?: { data?: { error?: string } } })?.response?.data?.error ??
        "Failed to reset password";
      message.error(msg);
    } finally {
      setRotatingId(null);
    }
  };

  const openWebmail = (row: MailboxRow) => {
    // Sever the opener link so the webmail tab can't reverse-tabnab the panel.
    // (noopener can't be passed to window.open here — it returns null and
    // breaks the synchronous-popup-then-set-href pattern that dodges blockers.)
    const popup = window.open("about:blank", "_blank");
    if (popup) {
      try {
        popup.opener = null;
      } catch {
        // older Safari — best effort
      }
    }
    ssoMutation.mutate(
      { id: row.id },
      {
        onSuccess: (data) => {
          if (popup && data?.url) popup.location.href = data.url;
        },
        onError: (err) => {
          const code = (err as { response?: { data?: { error?: string } } })?.response
            ?.data?.error;
          popup?.close();
          if (code === "sso_unavailable_rotate_password") {
            message.error(
              "Rotate the mailbox password first — SSO material is populated on rotation.",
            );
          } else {
            message.error("Failed to open webmail");
          }
        },
      },
    );
  };

  const loading = loadingDomains || mailboxResults.some((r) => r.isLoading);

  if (loading && rows.length === 0) {
    return <Skeleton active paragraph={{ rows: 4 }} />;
  }

  if (rows.length === 0) {
    return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="No mailboxes yet" />;
  }

  return (
    <>
      <Table<MailboxRow>
        scroll={{ x: "max-content" }}
        rowKey="id"
        loading={loading && rows.length === 0}
        dataSource={rows}
        pagination={{ pageSize: 20, showSizeChanger: true }}
        locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="No mailboxes" /> }}
        columns={[
          {
            title: "Email",
            dataIndex: "email",
            ellipsis: true,
            sorter: (a, b) => a.email.localeCompare(b.email),
            render: (v: string) => (
              <span
                style={{ display: "inline-flex", alignItems: "center", gap: 8 }}
              >
                <LibravatarAvatar email={v} size={24} />
                <Typography.Text style={{ fontFamily: "monospace" }}>
                  {v}
                </Typography.Text>
              </span>
            ),
          },
          {
            title: "Name",
            dataIndex: "display_name",
            ellipsis: true,
            render: (v: string) =>
              v ? (
                v
              ) : (
                <Typography.Text type="secondary">—</Typography.Text>
              ),
          },
          {
            title: "Domain",
            dataIndex: "domain_name",
            sorter: (a, b) => a.domain_name.localeCompare(b.domain_name),
            width: 220,
          },
          {
            title: "Groups",
            key: "groups",
            width: 200,
            render: (_: unknown, record: MailboxRow) => {
              const groups = groupsByMailbox[record.id] ?? [];
              if (groups.length === 0)
                return <Typography.Text type="secondary">—</Typography.Text>;
              return (
                <Space size={[0, 4]} wrap>
                  {groups.map((g) => (
                    <Tag key={g.group_id}>{g.group_name}</Tag>
                  ))}
                </Space>
              );
            },
          },
          {
            title: "Auto replies",
            key: "auto_replies",
            width: 110,
            align: "center" as const,
            render: (_: unknown, record: MailboxRow) => {
              const ar = arByMailbox[record.id];
              if (!ar?.enabled)
                return <Typography.Text type="secondary">—</Typography.Text>;
              const fmt = (d: string | null) =>
                d ? new Date(d).toLocaleDateString() : "—";
              const dates =
                ar.from_date || ar.to_date
                  ? `${fmt(ar.from_date)} → ${fmt(ar.to_date)}`
                  : "always on";
              return (
                <Tooltip
                  title={
                    <span>
                      <strong>{ar.subject || "(default subject)"}</strong>
                      <br />
                      {dates}
                      {ar.text_body ? (
                        <>
                          <br />
                          {ar.text_body.slice(0, 140)}
                          {ar.text_body.length > 140 ? "…" : ""}
                        </>
                      ) : null}
                    </span>
                  }
                >
                  <Button
                    type="text"
                    icon={<ClockCircleOutlined style={{ color: "#52c41a" }} />}
                    aria-label="Automatic replies active"
                    onClick={() => setArTarget(record)}
                  />
                </Tooltip>
              );
            },
          },
          {
            title: "Quota",
            dataIndex: "quota_bytes",
            width: 220,
            render: (quota: number, row) => {
              const used = row.last_usage_bytes ?? 0;
              const pct =
                quota > 0 ? Math.min(100, Math.round((used / quota) * 100)) : 0;
              return (
                <Tooltip title={`${formatBytes(used)} of ${formatBytes(quota)}`}>
                  <Progress
                    percent={pct}
                    size="small"
                    status={pct >= 90 ? "exception" : "normal"}
                    format={() => `${formatBytes(used)} / ${formatBytes(quota)}`}
                  />
                </Tooltip>
              );
            },
          },
          {
            title: "Last usage",
            dataIndex: "last_usage_at",
            width: 160,
            render: (v: string | null | undefined) =>
              v ? (
                new Date(v).toLocaleString()
              ) : (
                <Typography.Text type="secondary">never</Typography.Text>
              ),
          },
          {
            title: "Status",
            dataIndex: "is_disabled",
            width: 100,
            render: (disabled: boolean) =>
              disabled ? (
                <Tag color="red">disabled</Tag>
              ) : (
                <Tag color="green">active</Tag>
              ),
          },
          {
            title: "Actions",
            width: 120,
            render: (_, row) => {
              const menuItems: MenuProps["items"] = [
                {
                  key: "edit",
                  icon: <EditOutlined />,
                  label: "Edit",
                  onClick: () => setEditTarget(row),
                },
                {
                  key: "resetpw",
                  icon: <KeyOutlined />,
                  label: "Reset password",
                  onClick: () => openReset(row),
                },
                {
                  key: "autoreply",
                  icon: <ClockCircleOutlined />,
                  label: "Automatic replies",
                  onClick: () => setArTarget(row),
                },
                { type: "divider" },
                {
                  key: "remove",
                  icon: <DeleteOutlined />,
                  label: "Remove",
                  danger: true,
                  onClick: () => {
                    Modal.confirm({
                      title: `Delete ${row.email}?`,
                      content:
                        "All mail in this mailbox will be removed. This cannot be undone.",
                      okText: "Delete",
                      okType: "danger",
                      onOk: async () => {
                        try {
                          await deleteMutation.mutateAsync({
                            id: row.id,
                            domainId: row.domain_id,
                          });
                          message.success("Mailbox deleted");
                        } catch (err) {
                          const msg =
                            (err as { response?: { data?: { detail?: string } } })
                              ?.response?.data?.detail ?? "Failed to delete";
                          message.error(msg);
                        }
                      },
                    });
                  },
                },
              ];
              return (
                <Space>
                  <Tooltip title="Open webmail for this mailbox">
                    <Button
                      type="text"
                      icon={<MailOutlined />}
                      loading={
                        ssoMutation.isPending &&
                        ssoMutation.variables?.id === row.id
                      }
                      onClick={() => openWebmail(row)}
                    />
                  </Tooltip>
                  <Dropdown
                    trigger={["click"]}
                    placement="bottomRight"
                    menu={{ items: menuItems }}
                  >
                    <RowActionButton
                      icon={<MoreOutlined />}
                      color="default"
                      aria-label="Actions"
                    />
                  </Dropdown>
                </Space>
              );
            },
          },
        ]}
      />

      <EditMailboxModal
        open={editTarget !== null}
        mailbox={editTarget}
        onClose={() => setEditTarget(null)}
      />

      <AutoReplyModal
        open={arTarget !== null}
        mailboxId={arTarget?.id ?? ""}
        email={arTarget?.email ?? ""}
        current={arTarget ? arByMailbox[arTarget.id] ?? null : null}
        onClose={() => setArTarget(null)}
      />

      <Modal
        open={resetTarget !== null}
        title={resetTarget ? `Reset password — ${resetTarget.email}` : "Reset password"}
        okText="Set password"
        confirmLoading={resetTarget ? rotatingId === resetTarget.id : false}
        onOk={submitReset}
        onCancel={() => setResetTarget(null)}
        destroyOnClose
      >
        <Form form={resetForm} layout="vertical" requiredMark={false}>
          <Form.Item
            label="New password"
            name="password"
            tooltip="Leave blank to auto-generate. Auto-generated passwords are shown exactly once."
          >
            <PasswordInput
              autoComplete="new-password"
              placeholder="(leave blank to auto-generate)"
            />
          </Form.Item>
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            Use the dice button to generate a strong password, or type your own.
          </Typography.Text>
        </Form>
      </Modal>

      {passwordModal && (
        <DatabaseUserPasswordModal
          open={true}
          username={passwordModal.email}
          password={passwordModal.password}
          title={passwordModal.title}
          onClose={() => setPasswordModal(null)}
        />
      )}
    </>
  );
};
