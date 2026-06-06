// User API Tokens — list / mint / revoke for the signed-in tenant.
//
// Wire path:   GET    /api/v1/me/api-tokens
//              POST   /api/v1/me/api-tokens     {name, expires_in_seconds?, scopes?}
//                       → {token: {…}, secret: "jat_…"}  (plaintext shown ONCE)
//              DELETE /api/v1/me/api-tokens/:id
//
// The "shown once" rule is enforced server-side (the plaintext is
// never persisted), so this page MUST surface it loudly in a modal
// the user can't dismiss without copying. Anything else risks a
// "wait, what was that secret?" support ticket.

import { useEffect, useMemo, useState } from "react";
import {
  Alert,
  Button,
  Card,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Space,
  Table,
  Tabs,
  Tag,
  Tooltip,
  Typography,
  notification,
} from "antd";
import type { ColumnsType } from "antd/es/table";
import {
  CopyOutlined,
  DeleteOutlined,
  KeyOutlined,
  PlusOutlined,
} from "@ant-design/icons";
import { apiClient } from "../../../apiClient";
import { APIDocsPage } from "../../shared/APIDocsPage";

type UserAPIToken = {
  id: string;
  user_id: string;
  name: string;
  secret_last4: string;
  scopes: string[] | null;
  last_used_at: string | null;
  last_used_ip: string | null;
  expires_at: string | null;
  revoked_at: string | null;
  created_at: string;
};

type CreateResponse = {
  token: UserAPIToken;
  secret: string;
};

// fmtDate prints ISO timestamps as "YYYY-MM-DD HH:mm" UTC for
// consistency across browsers (Intl.DateTimeFormat locale formats
// drift between Chrome / Safari / Firefox locales).
function fmtDate(s: string | null): string {
  if (!s) return "—";
  const d = new Date(s);
  if (Number.isNaN(d.getTime())) return s;
  return (
    d.toISOString().slice(0, 16).replace("T", " ") + " UTC"
  );
}

// statusTag projects (revoked, expires_at) into a single coloured tag.
function statusTag(t: UserAPIToken): JSX.Element {
  if (t.revoked_at) {
    return <Tag color="default">Revoked</Tag>;
  }
  if (t.expires_at && new Date(t.expires_at).getTime() < Date.now()) {
    return <Tag color="orange">Expired</Tag>;
  }
  return <Tag color="green">Active</Tag>;
}

export function UserAPITokensPage(): JSX.Element {
  const [rows, setRows] = useState<UserAPIToken[]>([]);
  const [loading, setLoading] = useState(true);
  const [createOpen, setCreateOpen] = useState(false);
  const [secret, setSecret] = useState<{ secret: string; name: string } | null>(
    null,
  );
  const [createForm] = Form.useForm<{
    name: string;
    expires_in_seconds?: number;
  }>();

  const load = async () => {
    setLoading(true);
    try {
      const resp = await apiClient.get<{ items: UserAPIToken[] }>(
        "/me/api-tokens",
      );
      setRows(resp.data.items ?? []);
    } catch (err) {
      const e = err as { message?: string };
      notification.error({
        message: "Failed to load API tokens",
        description: e.message ?? "Unknown error",
      });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
  }, []);

  const onCreate = async () => {
    try {
      const values = await createForm.validateFields();
      const body: { name: string; expires_in_seconds?: number } = {
        name: values.name,
      };
      if (values.expires_in_seconds && values.expires_in_seconds > 0) {
        body.expires_in_seconds = values.expires_in_seconds;
      }
      const resp = await apiClient.post<CreateResponse>("/me/api-tokens", body);
      setSecret({ secret: resp.data.secret, name: resp.data.token.name });
      setCreateOpen(false);
      createForm.resetFields();
      void load();
    } catch (err) {
      // validateFields throws with a list of errors; nothing to toast.
      if ((err as { errorFields?: unknown }).errorFields) return;
      const e = err as { message?: string };
      notification.error({
        message: "Failed to create token",
        description: e.message ?? "Unknown error",
      });
    }
  };

  const onRevoke = async (t: UserAPIToken) => {
    try {
      await apiClient.delete(`/me/api-tokens/${t.id}`);
      notification.success({ message: `Token "${t.name}" revoked` });
      void load();
    } catch (err) {
      const e = err as { message?: string };
      notification.error({
        message: "Failed to revoke token",
        description: e.message ?? "Unknown error",
      });
    }
  };

  const copy = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      notification.success({ message: "Copied to clipboard" });
    } catch {
      notification.error({
        message: "Copy failed",
        description: "Select the secret and copy manually.",
      });
    }
  };

  const columns: ColumnsType<UserAPIToken> = useMemo(
    () => [
      {
        title: "Name",
        dataIndex: "name",
        render: (name: string, row) => (
          <Space>
            <KeyOutlined />
            <span>{name}</span>
            <Typography.Text type="secondary">
              (jat_…{row.secret_last4})
            </Typography.Text>
          </Space>
        ),
      },
      {
        title: "Status",
        render: (_: unknown, row) => statusTag(row),
      },
      {
        title: "Created",
        dataIndex: "created_at",
        render: fmtDate,
      },
      {
        title: "Last used",
        render: (_: unknown, row) => (
          <Tooltip
            title={
              row.last_used_ip
                ? `from ${row.last_used_ip}`
                : "no usage recorded"
            }
          >
            <span>{fmtDate(row.last_used_at)}</span>
          </Tooltip>
        ),
      },
      {
        title: "Expires",
        dataIndex: "expires_at",
        render: fmtDate,
      },
      {
        title: "Actions",
        render: (_: unknown, row) =>
          row.revoked_at ? (
            <Typography.Text type="secondary">
              revoked {fmtDate(row.revoked_at)}
            </Typography.Text>
          ) : (
            <Popconfirm
              title="Revoke this token?"
              description="Any script using it will start getting 401."
              okText="Revoke"
              okType="danger"
              onConfirm={() => onRevoke(row)}
            >
              <Button danger icon={<DeleteOutlined />} size="small">
                Revoke
              </Button>
            </Popconfirm>
          ),
      },
    ],
    [],
  );

  const tokensCard = (
    <Card
      title="API Tokens"
      extra={
        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={() => setCreateOpen(true)}
        >
          New token
        </Button>
      }
    >
      <Typography.Paragraph type="secondary" style={{ marginTop: 0 }}>
        Tokens authenticate API requests as you, scoped to resources
        you own. Use them to script DNS records, deploy WordPress,
        drive a DDNS updater on your router — anything the panel UI
        lets you do. Send the token as{" "}
        <Typography.Text code>Authorization: Bearer jat_…</Typography.Text>
        .
      </Typography.Paragraph>

      <Table<UserAPIToken>
        rowKey="id"
        loading={loading}
        dataSource={rows}
        columns={columns}
        pagination={false}
        scroll={{ x: "max-content" }}
      />
    </Card>
  );

  return (
    <>
      <Tabs
        defaultActiveKey="tokens"
        items={[
          { key: "tokens", label: "Tokens", children: tokensCard },
          { key: "docs", label: "API Docs", children: <APIDocsPage /> },
        ]}
      />

      <Modal
        title="Create API token"
        open={createOpen}
        onCancel={() => {
          setCreateOpen(false);
          createForm.resetFields();
        }}
        onOk={onCreate}
        okText="Create"
      >
        <Form form={createForm} layout="vertical">
          <Form.Item
            label="Name"
            name="name"
            rules={[
              { required: true, message: "Required" },
              { max: 100, message: "Max 100 characters" },
            ]}
            extra="A label for you — e.g. 'office router DDNS', 'CI deploy bot'."
          >
            <Input placeholder="My token" autoFocus maxLength={100} />
          </Form.Item>
          <Form.Item
            label="Expires in (seconds, optional)"
            name="expires_in_seconds"
            extra="Leave blank for a token that never expires. Max 1 year (31536000)."
            rules={[
              {
                type: "number",
                min: 60,
                max: 31536000,
                message: "60 to 31,536,000 (1 minute to 1 year)",
              },
            ]}
          >
            <InputNumber
              min={60}
              max={31536000}
              step={3600}
              style={{ width: "100%" }}
              placeholder="e.g. 86400 for 1 day"
            />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={`Token created: ${secret?.name ?? ""}`}
        open={!!secret}
        onCancel={() => setSecret(null)}
        onOk={() => setSecret(null)}
        okText="I've copied it"
        cancelButtonProps={{ style: { display: "none" } }}
        closable={false}
        maskClosable={false}
      >
        <Alert
          type="warning"
          showIcon
          message="Copy this secret now."
          description="The plaintext token is shown only once. After you close this dialog the panel only stores its hash — there's no way to retrieve it again. Lost? Revoke + create a new one."
          style={{ marginBottom: 16 }}
        />
        <Input.Group compact>
          <Input
            value={secret?.secret}
            readOnly
            onFocus={(e) => e.currentTarget.select()}
            style={{ width: "calc(100% - 90px)" }}
          />
          <Button
            type="primary"
            icon={<CopyOutlined />}
            onClick={() => secret && void copy(secret.secret)}
            style={{ width: 90 }}
          >
            Copy
          </Button>
        </Input.Group>
      </Modal>
    </>
  );
}
