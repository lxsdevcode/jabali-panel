// UserPHPPoolCard — GH #339 phase 1. Lets a tenant tune their own PHP-FPM
// pool(s) from the panel instead of the API. One card per PHP version the
// user has a pool for; the pm_max_children / idle-timeout caps mirror the
// server-side limits (phpPoolUserMaxChildrenCap=100, phpPoolMaxIdleTimeoutSec
// =86400 in php_pools.go) so the form fails fast before the API rejects it.
//
// The admin already had this via PHPPoolEdit; the /php-pools routes are
// user-scoped (owner sees + edits only their own pools), so this is UI-only.
import {
  Alert,
  Button,
  Card,
  Form,
  InputNumber,
  Select,
  Space,
  Spin,
  Typography,
  message,
} from "antd";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { apiClient } from "../../../apiClient";

// Mirror of php_pools.go caps for a tenant. Server enforces these regardless.
const USER_MAX_CHILDREN_CAP = 100;
const MAX_IDLE_TIMEOUT_SEC = 86400;

type PHPPool = {
  id: string;
  php_version: string;
  pm_mode: string;
  pm_max_children: number;
  process_idle_timeout_seconds: number;
};

const PM_MODES = [
  { value: "dynamic", label: "dynamic — scale workers with load (default)" },
  { value: "ondemand", label: "ondemand — spawn on request, idle out" },
  { value: "static", label: "static — a fixed number of workers" },
];

const PoolForm = ({ pool }: { pool: PHPPool }) => {
  const qc = useQueryClient();
  const [form] = Form.useForm<Omit<PHPPool, "id" | "php_version">>();
  const [saving, setSaving] = useState(false);

  const onSave = async () => {
    const vals = await form.validateFields();
    setSaving(true);
    try {
      await apiClient.put(`/php-pools/${pool.id}`, {
        pm_mode: vals.pm_mode,
        pm_max_children: vals.pm_max_children,
        process_idle_timeout_seconds: vals.process_idle_timeout_seconds,
        php_version: pool.php_version,
      });
      message.success(`PHP ${pool.php_version} pool updated`);
      qc.invalidateQueries({ queryKey: ["user-php-pools"] });
    } catch (err) {
      const detail =
        (err as { response?: { data?: { message?: string; error?: string } } })
          ?.response?.data?.message ??
        (err as { response?: { data?: { error?: string } } })?.response?.data
          ?.error ??
        "Update failed";
      message.error(detail);
    } finally {
      setSaving(false);
    }
  };

  return (
    <Form
      form={form}
      layout="vertical"
      initialValues={{
        pm_mode: pool.pm_mode,
        pm_max_children: pool.pm_max_children,
        process_idle_timeout_seconds: pool.process_idle_timeout_seconds,
      }}
      style={{ maxWidth: 520 }}
    >
      <Typography.Title level={5} style={{ marginTop: 0 }}>
        PHP {pool.php_version}
      </Typography.Title>
      <Form.Item name="pm_mode" label="Process manager (pm)">
        <Select options={PM_MODES} />
      </Form.Item>
      <Form.Item
        name="pm_max_children"
        label="Max children (worker processes)"
        rules={[
          {
            type: "number",
            min: 1,
            max: USER_MAX_CHILDREN_CAP,
            message: `1–${USER_MAX_CHILDREN_CAP} on a shared plan`,
          },
        ]}
      >
        <InputNumber min={1} max={USER_MAX_CHILDREN_CAP} style={{ width: 160 }} />
      </Form.Item>
      <Form.Item
        name="process_idle_timeout_seconds"
        label="Idle timeout (seconds) — ondemand/dynamic only"
        rules={[
          {
            type: "number",
            min: 1,
            max: MAX_IDLE_TIMEOUT_SEC,
            message: `1–${MAX_IDLE_TIMEOUT_SEC}`,
          },
        ]}
      >
        <InputNumber
          min={1}
          max={MAX_IDLE_TIMEOUT_SEC}
          style={{ width: 160 }}
        />
      </Form.Item>
      <Button type="primary" loading={saving} onClick={onSave}>
        Save PHP {pool.php_version} pool
      </Button>
    </Form>
  );
};

export const UserPHPPoolCard = () => {
  const { data, isLoading, isError } = useQuery({
    queryKey: ["user-php-pools"],
    queryFn: async () => {
      const resp = await apiClient.get<{ data: PHPPool[] }>("/php-pools");
      return resp.data.data ?? [];
    },
  });

  return (
    <Card title="PHP-FPM Pool (advanced)" style={{ marginTop: 16 }}>
      <Typography.Paragraph type="secondary">
        Tune the worker pool your sites run under. These apply to every domain
        on the given PHP version. Limits are capped for shared-hosting safety
        (max {USER_MAX_CHILDREN_CAP} children).
      </Typography.Paragraph>
      {isLoading && <Spin />}
      {isError && (
        <Alert type="error" message="Could not load your PHP-FPM pools." />
      )}
      {!isLoading && !isError && (!data || data.length === 0) && (
        <Alert
          type="info"
          message="No PHP-FPM pool yet — it's created the first time a domain is assigned a PHP version."
        />
      )}
      {!isLoading && data && data.length > 0 && (
        <Space direction="vertical" size="large" style={{ width: "100%" }}>
          {data.map((pool) => (
            <PoolForm key={pool.id} pool={pool} />
          ))}
        </Space>
      )}
    </Card>
  );
};
