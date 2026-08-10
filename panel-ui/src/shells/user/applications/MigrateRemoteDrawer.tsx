import { useEffect, useState } from "react";
import { Alert, Button, Card, Drawer, Form, Input, Select, Space, Steps, Spin, Radio, Tabs, Typography } from "antd";
import { feedback } from "../../../lib/feedback"; // GH #970: themed toasts
import { CloudServerOutlined, ApiOutlined } from "@icons";
import { apiClient } from "../../../apiClient";

type Kind = "wordpress_ssh" | "wordpress_plugin";

type Props = {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
};

type DomainOpt = { label: string; value: string };

// Tenant WordPress migration wizard. Drives the owner-scoped /migrations/* API:
// create -> secrets -> pull-source -> (poll) -> import-wp. A tenant may only
// migrate into a domain they own (the API enforces it; the UI lists their own).
export function MigrateRemoteDrawer({ open, onClose, onSuccess }: Props) {
  const [step, setStep] = useState(0);
  const [kind, setKind] = useState<Kind | null>(null);
  const [jobId, setJobId] = useState<string | null>(null);
  const [destDomain, setDestDomain] = useState<string>("");
  const [domains, setDomains] = useState<DomainOpt[]>([]);
  const [busy, setBusy] = useState(false);

  const [createForm] = Form.useForm();
  const [secretForm] = Form.useForm();

  const reset = () => {
    setStep(0);
    setKind(null);
    setJobId(null);
    setDestDomain("");
    createForm.resetFields();
    secretForm.resetFields();
  };

  useEffect(() => {
    if (!open) return;
    reset();
    apiClient
      .get<{ data?: Array<{ name: string }> }>("/domains?page_size=200")
      .then((r) => {
        const list = r.data?.data ?? [];
        setDomains(list.map((d) => ({ label: d.name, value: d.name })));
      })
      .catch(() => setDomains([]));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  useEffect(() => {
    if (step === 3 && jobId && !verify && !verifying) {
      void handleVerify();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [step]);

  // Step 1 — create the job.
  const handleCreate = async (v: Record<string, string>) => {
    setBusy(true);
    try {
      const { data } = await apiClient.post<{ id: string }>(
        "/migrations/wordpress",
        {
          source_kind: kind,
          source_host: v.source_host,
          source_user: v.source_user,
          source_path: v.source_path,
          dest_domain: v.dest_domain,
        },
      );
      setJobId(data.id);
      setDestDomain(v.dest_domain);
      setStep(2);
    } catch (e) {
      feedback.message.error(errText(e) ?? "Create failed");
    } finally {
      setBusy(false);
    }
  };

  // Step 2 — save credentials (SSH creds or plugin token).
  const handleSecrets = async (v: Record<string, string>) => {
    if (!jobId) return;
    setBusy(true);
    try {
      await apiClient.post(`/migrations/${jobId}/secrets`, v);
      setStep(3);
    } catch (e) {
      feedback.message.error(errText(e) ?? "Failed to save credentials");
    } finally {
      setBusy(false);
    }
  };

  // Step 3 — kick the migration; it runs server-side (pull + auto-import) as a
  // background job, so the user can close the drawer immediately.
  const [started, setStarted] = useState(false);
  const [installs, setInstalls] = useState<{ root: string; siteurl?: string }[]>([]);
  const [scanning, setScanning] = useState(false);
  type Verify = {
    ok?: boolean;
    error?: string;
    siteurl?: string;
    wp_version?: string;
    file_count?: number;
    db_bytes?: number;
    needs_update?: boolean;
    plugin_version?: string;
  };
  const [verify, setVerify] = useState<Verify | null>(null);
  const [verifying, setVerifying] = useState(false);
  const handleVerify = async () => {
    if (!jobId) return;
    setVerifying(true);
    setVerify(null);
    try {
      const { data } = await apiClient.post<Verify>(`/migrations/${jobId}/verify`, {});
      setVerify(data);
    } catch (e) {
      setVerify({ ok: false, error: errText(e) ?? "Verification failed" });
    } finally {
      setVerifying(false);
    }
  };

  const handleScan = async () => {
    if (!jobId) return;
    setScanning(true);
    try {
      const { data } = await apiClient.post<{ installs: { root: string; siteurl?: string }[] }>(
        `/migrations/${jobId}/scan-wp`,
        {},
      );
      setInstalls(data.installs ?? []);
      if (!data.installs?.length) feedback.message.info("No WordPress installs found on the source.");
    } catch (e) {
      feedback.message.error(errText(e) ?? "Scan failed");
    } finally {
      setScanning(false);
    }
  };
  const pickInstall = async (root: string) => {
    if (!jobId) return;
    try {
      await apiClient.post(`/migrations/${jobId}/source-path`, { source_path: root });
      feedback.message.success("Selected " + root);
    } catch (e) {
      feedback.message.error(errText(e) ?? "Could not select");
    }
  };

  const handleStart = async () => {
    if (!jobId) return;
    setBusy(true);
    try {
      await apiClient.post(`/migrations/${jobId}/pull-source`, { ssh_user: "root" });
      setStarted(true);
      feedback.message.success("Migration started in the background.");
    } catch (e) {
      feedback.message.error(errText(e) ?? "Failed to start the migration");
    } finally {
      setBusy(false);
    }
  };

  return (
    <Drawer
      title="Migrate WordPress from a remote site"
      open={open}
      onClose={onClose}
      width={560}
      destroyOnClose
    >
      <Steps
        size="small"
        current={step}
        style={{ marginBottom: 24 }}
        items={[
          { title: "Source" },
          { title: "Details" },
          { title: "Credentials" },
          { title: "Start" },
        ]}
      />

      {step === 0 && (
        <Space direction="vertical" size="large" style={{ width: "100%" }}>
          <Typography.Paragraph type="secondary">
            Bring an existing WordPress site into this account. Choose how to reach the source.
          </Typography.Paragraph>
          <Card
            hoverable
            onClick={() => {
              setKind("wordpress_ssh");
              setStep(1);
            }}
          >
            <Space>
              <CloudServerOutlined style={{ fontSize: 22 }} />
              <div>
                <Typography.Text strong>SSH</Typography.Text>
                <div>
                  <Typography.Text type="secondary">
                    Cloudways / VPS / generic host — Jabali connects by SSH.
                  </Typography.Text>
                </div>
              </div>
            </Space>
          </Card>
          <Card
            hoverable
            onClick={() => {
              setKind("wordpress_plugin");
              setStep(1);
            }}
          >
            <Space>
              <ApiOutlined style={{ fontSize: 22 }} />
              <div>
                <Typography.Text strong>WordPress plugin</Typography.Text>
                <div>
                  <Typography.Text type="secondary">
                    No SSH — install the jabali-migrator plugin on the source, paste a token.
                  </Typography.Text>
                </div>
              </div>
            </Space>
          </Card>
        </Space>
      )}

      {step === 1 && (
        <Form form={createForm} layout="vertical" onFinish={handleCreate}>
          {kind === "wordpress_plugin" ? (
            <Form.Item
              name="source_host"
              label="Source site URL"
              rules={[{ required: true, message: "Source URL required" }]}
            >
              <Input placeholder="https://old-site.com" />
            </Form.Item>
          ) : (
            <>
              <Form.Item
                name="source_host"
                label="Source SSH host"
                rules={[{ required: true, message: "Host required" }]}
              >
                <Input placeholder="old-host.com or 203.0.113.5" />
              </Form.Item>
              <Form.Item
                name="source_user"
                label="SSH user"
                rules={[{ required: true, message: "SSH user required" }]}
              >
                <Input placeholder="root" />
              </Form.Item>
              <Form.Item
                name="source_path"
                label="WordPress path (optional)"
                tooltip="Absolute path to the WP root (must contain wp-config.php). Blank = auto-detect ~/public_html, /home/*/public_html, Cloudways."
              >
                <Input placeholder="/home/master/applications/xxxx/public_html" />
              </Form.Item>
            </>
          )}
          <Form.Item
            name="dest_domain"
            label="Destination domain (optional)"
            tooltip="Leave blank to auto-detect the source site's own domain and create it here. Or pick one of your existing domains — the site imports into /home/<you>/domains/<domain>/public_html."
          >
            <Select
              options={domains}
              showSearch
              allowClear
              placeholder="auto-detect from the source (or pick your domain)"
            />
          </Form.Item>
          <Space>
            <Button onClick={() => setStep(0)}>Back</Button>
            <Button type="primary" htmlType="submit" loading={busy}>
              Continue
            </Button>
          </Space>
        </Form>
      )}

      {step === 2 && (
        <Form form={secretForm} layout="vertical" onFinish={handleSecrets}>
          {kind === "wordpress_plugin" ? (
            <>
              <Alert
                type="info"
                showIcon
                style={{ marginBottom: 16 }}
                message="Migration token"
                description="On the source site install the jabali-migrator plugin (Tools → Jabali Migrator), generate a token, and paste it here."
              />
              <Form.Item
                name="plugin_token"
                label="Token"
                rules={[{ required: true, message: "Token required" }]}
              >
                <Input.Password placeholder="64-char token" autoComplete="off" />
              </Form.Item>
            </>
          ) : (
            <Tabs
              defaultActiveKey="password"
              items={[
                {
                  key: "password",
                  label: "Password",
                  children: (
                    <Form.Item name="ssh_password" label="SSH password">
                      <Input.Password autoComplete="off" />
                    </Form.Item>
                  ),
                },
                {
                  key: "key",
                  label: "Private key",
                  children: (
                    <Form.Item name="ssh_private_key" label="PEM private key">
                      <Input.TextArea rows={6} style={{ fontFamily: "monospace", fontSize: 12 }} />
                    </Form.Item>
                  ),
                },
              ]}
            />
          )}
          <Button type="primary" htmlType="submit" loading={busy}>
            Save + continue
          </Button>
        </Form>
      )}

      {step === 3 && (
        <Space direction="vertical" size="large" style={{ width: "100%" }}>
          {!started && (
            <Card size="small" title="Pre-flight check">
              {verifying ? (
                <Space>
                  <Spin size="small" /> <Typography.Text>Handshaking with the source…</Typography.Text>
                </Space>
              ) : verify?.ok ? (
                <Space direction="vertical" style={{ width: "100%" }}>
                  <Alert
                    type="success"
                    showIcon
                    message="Source reachable"
                    description={
                      <span>
                        {verify.siteurl}
                        {verify.wp_version ? ` · WP ${verify.wp_version}` : ""}
                        {verify.file_count ? ` · ${verify.file_count} files` : ""}
                        {verify.db_bytes ? ` · DB ${(verify.db_bytes / 1048576).toFixed(1)} MB` : ""}
                      </span>
                    }
                  />
                  {verify.needs_update && (
                    <Alert
                      type="warning"
                      showIcon
                      message="Source plugin is outdated"
                      description={`The jabali-migrator plugin on the source is ${verify.plugin_version}. Update it to 0.1.2+ or the database export will fail. Then re-check.`}
                    />
                  )}
                </Space>
              ) : (
                <Space direction="vertical" style={{ width: "100%" }}>
                  <Alert type="error" showIcon message="Cannot start yet" description={verify?.error ?? "Source check failed"} />
                  <Button onClick={handleVerify} loading={verifying}>
                    Re-check
                  </Button>
                </Space>
              )}
            </Card>
          )}
          {!started && kind === "wordpress_ssh" && (
            <Card size="small" title="Scan for WordPress installations (optional)">
              <Space direction="vertical" style={{ width: "100%" }}>
                <Typography.Text type="secondary">
                  Search /var/www and /home on the source and pick which install to migrate.
                </Typography.Text>
                <Button loading={scanning} onClick={handleScan}>
                  Scan the source
                </Button>
                {installs.length > 0 && (
                  <Radio.Group
                    style={{ width: "100%" }}
                    onChange={(e) => void pickInstall(e.target.value)}
                  >
                    <Space direction="vertical" style={{ width: "100%" }}>
                      {installs.map((i) => (
                        <Radio key={i.root} value={i.root}>
                          <Typography.Text code>{i.root}</Typography.Text>
                          {i.siteurl ? <Typography.Text type="secondary"> — {i.siteurl}</Typography.Text> : null}
                        </Radio>
                      ))}
                    </Space>
                  </Radio.Group>
                )}
              </Space>
            </Card>
          )}
          {!started ? (
            <>
              <Alert
                type="info"
                showIcon
                message="Start the migration"
                description="Jabali pulls the database + files from the source and imports them into your domain. This runs as a background job — you can close this window and the site will appear in Applications when it finishes."
              />
              <Button
                type="primary"
                loading={busy}
                disabled={!verify?.ok || verify?.needs_update}
                onClick={handleStart}
              >
                Start migration
              </Button>
            </>
          ) : (
            <>
              <Alert
                type="success"
                showIcon
                message="Migration running in the background"
                description={`It will import into ${destDomain} when the transfer completes. You can close this window and keep working — the site appears in Applications when done.`}
              />
              <Button
                type="primary"
                onClick={() => {
                  onSuccess();
                  onClose();
                }}
              >
                Close
              </Button>
            </>
          )}
        </Space>
      )}

    </Drawer>
  );
}

function errText(e: unknown): string | undefined {
  return (e as { response?: { data?: { detail?: string; error?: string } } })?.response?.data
    ?.detail ?? (e as { response?: { data?: { error?: string } } })?.response?.data?.error;
}
