// MailboxForwardingSection — manage a mailbox's aliases and external
// forwards inline (GH #237, Microsoft 365-style). Embedded in the Edit
// Mailbox modal. Aliases are additional addresses that deliver to this
// mailbox; forwards redirect to an external address, optionally keeping a
// copy in the mailbox.
import { useMemo, useState } from "react";
import {
  App,
  Button,
  Checkbox,
  Divider,
  Input,
  List,
  Space,
  Tag,
  Tooltip,
  Typography,
} from "antd";
import { DeleteOutlined, PlusOutlined } from "@icons";

import {
  useForwarders,
  useCreateForwarder,
  useDeleteForwarder,
} from "../../hooks/useForwarders";

interface MailboxForwardingSectionProps {
  mailboxId: string;
  /** The mailbox's own domain, used to render alias addresses. */
  domainName: string;
}

export function MailboxForwardingSection({
  mailboxId,
  domainName,
}: MailboxForwardingSectionProps) {
  const { message } = App.useApp();
  const { data: all = [], isLoading } = useForwarders();
  const createMut = useCreateForwarder();
  const deleteMut = useDeleteForwarder();

  const { aliases, forwards } = useMemo(() => {
    const mine = all.filter((f) => f.mailbox_id === mailboxId);
    return {
      aliases: mine.filter((f) => f.type === "alias"),
      forwards: mine.filter((f) => f.type === "external"),
    };
  }, [all, mailboxId]);

  const [aliasLocal, setAliasLocal] = useState("");
  const [fwdTarget, setFwdTarget] = useState("");
  const [fwdKeepCopy, setFwdKeepCopy] = useState(false);

  const addAlias = async () => {
    const lp = aliasLocal.trim().toLowerCase();
    if (!lp) return;
    try {
      await createMut.mutateAsync({
        mailboxID: mailboxId,
        type: "alias",
        localPart: lp,
        target: "",
      });
      setAliasLocal("");
      message.success("Alias added");
    } catch (err) {
      message.error(errMsg(err, "Failed to add alias"));
    }
  };

  const addForward = async () => {
    const t = fwdTarget.trim();
    if (!t) return;
    try {
      await createMut.mutateAsync({
        mailboxID: mailboxId,
        type: "external",
        target: t,
        keepCopy: fwdKeepCopy,
      });
      setFwdTarget("");
      setFwdKeepCopy(false);
      message.success("Forwarding added");
    } catch (err) {
      message.error(errMsg(err, "Failed to add forwarding"));
    }
  };

  const remove = async (id: string) => {
    try {
      await deleteMut.mutateAsync(id);
      message.success("Removed");
    } catch (err) {
      message.error(errMsg(err, "Failed to remove"));
    }
  };

  return (
    <div>
      <Divider style={{ margin: "8px 0 16px" }} />
      <Typography.Text strong>Aliases</Typography.Text>
      <Typography.Paragraph type="secondary" style={{ fontSize: 12, marginBottom: 8 }}>
        Extra addresses that deliver to this mailbox.
      </Typography.Paragraph>
      <List
        size="small"
        loading={isLoading}
        locale={{ emptyText: "No aliases" }}
        dataSource={aliases}
        renderItem={(a) => (
          <List.Item
            actions={[
              <Button
                key="del"
                type="text"
                danger
                size="small"
                icon={<DeleteOutlined />}
                onClick={() => remove(a.id)}
              />,
            ]}
          >
            <Typography.Text style={{ fontFamily: "monospace" }}>
              {a.local_part}@{domainName}
            </Typography.Text>
          </List.Item>
        )}
      />
      <Space.Compact style={{ width: "100%", marginTop: 8 }}>
        <Input
          placeholder="alias (local part)"
          value={aliasLocal}
          onChange={(e) => setAliasLocal(e.target.value)}
          onPressEnter={addAlias}
          addonAfter={`@${domainName}`}
        />
        <Button
          icon={<PlusOutlined />}
          onClick={addAlias}
          loading={createMut.isPending}
        >
          Add
        </Button>
      </Space.Compact>

      <Divider style={{ margin: "16px 0" }} />
      <Typography.Text strong>Forwarding</Typography.Text>
      <Typography.Paragraph type="secondary" style={{ fontSize: 12, marginBottom: 8 }}>
        Send incoming mail on to another address.
      </Typography.Paragraph>
      <List
        size="small"
        locale={{ emptyText: "Not forwarding" }}
        dataSource={forwards}
        renderItem={(f) => (
          <List.Item
            actions={[
              <Button
                key="del"
                type="text"
                danger
                size="small"
                icon={<DeleteOutlined />}
                onClick={() => remove(f.id)}
              />,
            ]}
          >
            <Space>
              <Typography.Text style={{ fontFamily: "monospace" }}>
                → {f.target}
              </Typography.Text>
              {f.keep_copy ? (
                <Tooltip title="A copy is kept in this mailbox">
                  <Tag color="blue">keeps copy</Tag>
                </Tooltip>
              ) : null}
            </Space>
          </List.Item>
        )}
      />
      <Space.Compact style={{ width: "100%", marginTop: 8 }}>
        <Input
          placeholder="forward to (email address)"
          value={fwdTarget}
          onChange={(e) => setFwdTarget(e.target.value)}
          onPressEnter={addForward}
        />
        <Button
          icon={<PlusOutlined />}
          onClick={addForward}
          loading={createMut.isPending}
        >
          Add
        </Button>
      </Space.Compact>
      <Checkbox
        style={{ marginTop: 8 }}
        checked={fwdKeepCopy}
        onChange={(e) => setFwdKeepCopy(e.target.checked)}
      >
        Keep a copy of forwarded email in this mailbox
      </Checkbox>
    </div>
  );
}

function errMsg(err: unknown, fallback: string): string {
  return (
    (err as { response?: { data?: { error?: string; detail?: string } } })?.response
      ?.data?.error ??
    (err as { response?: { data?: { detail?: string } } })?.response?.data?.detail ??
    fallback
  );
}
