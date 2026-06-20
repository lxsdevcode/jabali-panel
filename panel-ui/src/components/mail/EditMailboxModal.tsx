// EditMailboxModal — edit a mailbox's display name (GH #197), quota, and
// enabled state. Shared by the user Mail tab and the admin server-wide
// Mail tab. Only changed fields are sent (partial PATCH /mailboxes/:id).
import { App, Form, InputNumber, Input, Modal, Switch, Typography } from "antd";
import { useEffect } from "react";

import { useUpdateMailbox, type Mailbox } from "../../hooks/useMailboxes";
import { MailboxForwardingSection } from "./MailboxForwardingSection";

const MIB = 1024 * 1024;
const QUOTA_MIN_MIB = 16;

interface EditMailboxModalProps {
  open: boolean;
  mailbox: Mailbox | null;
  onClose: () => void;
}

type FormValues = {
  display_name: string;
  quota_mib: number;
  enabled: boolean;
};

export function EditMailboxModal({ open, mailbox, onClose }: EditMailboxModalProps) {
  const { message } = App.useApp();
  const [form] = Form.useForm<FormValues>();
  const update = useUpdateMailbox();

  useEffect(() => {
    if (open && mailbox) {
      form.setFieldsValue({
        display_name: mailbox.display_name ?? "",
        quota_mib: Math.round(mailbox.quota_bytes / MIB),
        enabled: !mailbox.is_disabled,
      });
    }
  }, [open, mailbox, form]);

  const submit = async () => {
    if (!mailbox) return;
    const v = await form.validateFields();
    const patch: {
      id: string;
      domainId?: string;
      display_name?: string;
      quota_bytes?: number;
      is_disabled?: boolean;
    } = { id: mailbox.id, domainId: mailbox.domain_id };

    const nextName = (v.display_name ?? "").trim();
    if (nextName !== (mailbox.display_name ?? "")) patch.display_name = nextName;

    const nextQuota = Math.round(v.quota_mib) * MIB;
    if (nextQuota !== mailbox.quota_bytes) patch.quota_bytes = nextQuota;

    const nextDisabled = !v.enabled;
    if (nextDisabled !== mailbox.is_disabled) patch.is_disabled = nextDisabled;

    if (
      patch.display_name === undefined &&
      patch.quota_bytes === undefined &&
      patch.is_disabled === undefined
    ) {
      onClose();
      return;
    }

    try {
      await update.mutateAsync(patch);
      message.success("Mailbox updated");
      onClose();
    } catch (err) {
      const detail =
        (err as { response?: { data?: { detail?: string; error?: string } } })?.response
          ?.data?.detail ??
        (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
      message.error(detail ?? "Failed to update mailbox");
    }
  };

  return (
    <Modal
      open={open}
      title={mailbox ? `Edit ${mailbox.email}` : "Edit mailbox"}
      okText="Save"
      confirmLoading={update.isPending}
      onOk={submit}
      onCancel={onClose}
      destroyOnClose
    >
      <Form form={form} layout="vertical" requiredMark={false}>
        <Form.Item
          label="Display name"
          name="display_name"
          tooltip="Shown as the sender name in webmail and outgoing mail. Optional."
        >
          <Input placeholder="Alice Smith" maxLength={255} autoComplete="off" />
        </Form.Item>
        <Form.Item
          label="Quota (MiB)"
          name="quota_mib"
          rules={[{ required: true, message: "Quota is required" }]}
        >
          <InputNumber min={QUOTA_MIN_MIB} max={1024 * 1024} step={256} style={{ width: 200 }} />
        </Form.Item>
        <Form.Item label="Enabled" name="enabled" valuePropName="checked">
          <Switch />
        </Form.Item>
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          A display-name change applies to webmail on the mailbox&apos;s next login;
          an existing webmail identity may keep the old name until then.
        </Typography.Text>
        {mailbox ? (
          <MailboxForwardingSection
            mailboxId={mailbox.id}
            domainName={mailbox.email.split("@")[1] ?? ""}
          />
        ) : null}
      </Form>
    </Modal>
  );
}
