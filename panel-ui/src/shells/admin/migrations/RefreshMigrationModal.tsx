// RefreshMigrationModal — GH #646. Panel-driven guard-railed re-migration:
// POST /admin/migrations/refresh (backup-first, force-required). Mirrors the
// `jabali migrate refresh` CLI. The operator supplies the dest + staged-source
// paths; the backend takes a snapshot + DB dump BEFORE overwriting.
import { App, Alert, Form, Input, Modal, Typography } from "antd";
import { useState } from "react";

import { apiClient } from "../../../apiClient";
import { extractApiError } from "../../../apiErrors";

export function RefreshMigrationModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { message } = App.useApp();
  const [form] = Form.useForm();
  const [saving, setSaving] = useState(false);

  const submit = async () => {
    let v;
    try {
      v = await form.validateFields();
    } catch {
      return;
    }
    setSaving(true);
    try {
      const { data } = await apiClient.post<{ snapshot?: string; db_dump?: string }>(
        "/admin/migrations/refresh",
        { ...v, force: true },
      );
      message.success(`Refresh complete. Backup snapshot: ${data.snapshot ?? "?"} (remove once verified)`);
      onClose();
      form.resetFields();
    } catch (e) {
      message.error(extractApiError(e) ?? "Refresh failed (dest untouched if the backup failed)");
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal
      title="Refresh (re-pull) an already-migrated account"
      open={open}
      onCancel={onClose}
      onOk={() => void submit()}
      okText="Refresh (overwrites — backup taken first)"
      okButtonProps={{ danger: true, loading: saving }}
      width={640}
    >
      <Alert
        type="warning"
        showIcon
        style={{ marginBottom: 16 }}
        message="Destructive — overwrites the live account"
        description="Mirrors the staged source over the dest (--delete, preserving wp-config.php + jabali drop-ins) and drops+reimports the DB. A hardlink snapshot + DB dump are taken FIRST; the refresh aborts if that backup fails."
      />
      <Form form={form} layout="vertical">
        <Typography.Text strong>Destination</Typography.Text>
        <Form.Item name="docroot" label="Dest docroot" rules={[{ required: true }]}>
          <Input placeholder="/home/<user>/domains/<domain>/public_html" />
        </Form.Item>
        <Form.Item name="db" label="Dest DB name" rules={[{ required: true }]}>
          <Input placeholder="user_wp_db (identity unchanged)" />
        </Form.Item>
        <Form.Item name="os_user" label="Dest Linux user" rules={[{ required: true }]}>
          <Input placeholder="username" />
        </Form.Item>
        <Form.Item name="domain" label="Domain (cache purge)">
          <Input placeholder="example.com" />
        </Form.Item>
        <Typography.Text strong>Staged source</Typography.Text>
        <Form.Item name="source_docroot" label="Source docroot (staged)" rules={[{ required: true }]}>
          <Input placeholder="/var/lib/jabali-migrations/<job>/files" />
        </Form.Item>
        <Form.Item name="source_sql" label="Source SQL dump (staged)" rules={[{ required: true }]}>
          <Input placeholder="/var/lib/jabali-migrations/<job>/db.sql" />
        </Form.Item>
        <Form.Item name="old_url" label="Old URL (optional, search-replace)">
          <Input placeholder="https://old.example.com" />
        </Form.Item>
        <Form.Item name="new_url" label="New URL (optional)">
          <Input placeholder="https://example.com" />
        </Form.Item>
      </Form>
    </Modal>
  );
}
