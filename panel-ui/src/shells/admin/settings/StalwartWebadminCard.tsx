// StalwartWebadminCard — GH #243 (ADR-0142). Opt-in toggle that exposes
// Stalwart's WebAdmin UI through an nginx reverse-proxy (TLS + basic-auth +
// optional IP allowlist). Stalwart itself stays loopback-bound; this is the
// only door. Default off. Lives on the Server Settings → Email tab.
import { App, Alert, Button, Card, Input, Modal, Space, Switch, Typography } from "antd";
import { useEffect, useState } from "react";

import { apiClient } from "../../../apiClient";

interface WebadminSettings {
  stalwart_webadmin_enabled: boolean;
  stalwart_webadmin_allow_cidrs: string;
  hostname: string;
}

interface WebadminCredential {
  user: string;
  password: string;
  url: string;
}

export function StalwartWebadminCard() {
  const { message } = App.useApp();
  const [enabled, setEnabled] = useState(false);
  const [cidrs, setCidrs] = useState("");
  const [hostname, setHostname] = useState("");
  const [busy, setBusy] = useState(false);
  const [cred, setCred] = useState<WebadminCredential | null>(null);

  const refresh = async () => {
    try {
      const r = await apiClient.get<WebadminSettings>("/admin/settings");
      setEnabled(!!r.data.stalwart_webadmin_enabled);
      setCidrs(r.data.stalwart_webadmin_allow_cidrs || "");
      setHostname(r.data.hostname || "");
    } catch (err) {
      message.error(`Could not load WebAdmin setting: ${err instanceof Error ? err.message : String(err)}`);
    }
  };
  useEffect(() => {
    void refresh();
  }, []);

  const patch = async (body: Record<string, unknown>) => {
    const r = await apiClient.patch<{ stalwart_webadmin_credential?: WebadminCredential }>(
      "/admin/settings",
      body,
    );
    if (r.data?.stalwart_webadmin_credential) {
      setCred(r.data.stalwart_webadmin_credential);
    }
  };

  const doEnable = async () => {
    setBusy(true);
    try {
      await patch({ stalwart_webadmin_enabled: true, stalwart_webadmin_allow_cidrs: cidrs });
      setEnabled(true);
      message.success("Stalwart WebAdmin exposed");
    } catch (err) {
      const e = err as { response?: { data?: { detail?: string; error?: string } } };
      message.error(e.response?.data?.detail ?? e.response?.data?.error ?? "Failed to enable");
    } finally {
      setBusy(false);
    }
  };

  const onToggle = (val: boolean) => {
    if (val) {
      Modal.confirm({
        title: "Expose the Stalwart WebAdmin to the internet?",
        okText: "Enable",
        okButtonProps: { danger: true },
        content:
          "This publishes the full mail-server admin UI at admin." +
          (hostname || "<panel-hostname>") +
          " behind TLS + a basic-auth gateway. It is the highest-value target on the box — anyone who gets past the gateway can read all mail and send as any domain. Use the IP allowlist, keep the gateway credential safe, and turn this off when you are done.",
        onOk: doEnable,
      });
    } else {
      setBusy(true);
      patch({ stalwart_webadmin_enabled: false })
        .then(() => {
          setEnabled(false);
          message.warning("Stalwart WebAdmin proxy removed");
        })
        .catch((err) => message.error(err instanceof Error ? err.message : "Failed to disable"))
        .finally(() => setBusy(false));
    }
  };

  const saveAllowlist = async () => {
    setBusy(true);
    try {
      await patch({ stalwart_webadmin_allow_cidrs: cidrs });
      message.success("Allowlist saved");
    } catch (err) {
      const e = err as { response?: { data?: { detail?: string; error?: string } } };
      message.error(e.response?.data?.detail ?? e.response?.data?.error ?? "Failed to save allowlist");
    } finally {
      setBusy(false);
    }
  };

  const regenerate = async () => {
    setBusy(true);
    try {
      await patch({ stalwart_webadmin_regenerate: true });
      message.success("New gateway credential generated");
    } catch (err) {
      const e = err as { response?: { data?: { detail?: string; error?: string } } };
      message.error(e.response?.data?.detail ?? e.response?.data?.error ?? "Failed to regenerate");
    } finally {
      setBusy(false);
    }
  };

  return (
    <Card title="Stalwart WebAdmin (advanced)" size="small" style={{ marginTop: 16 }}>
      <Space direction="vertical" style={{ width: "100%" }} size="middle">
        <Typography.Paragraph type="secondary" style={{ margin: 0 }}>
          Exposes Stalwart&apos;s native admin UI (queues, tracing, fine-grained
          settings the panel doesn&apos;t surface) through nginx. Off by default —
          Stalwart stays bound to localhost; this is the only externally reachable
          door, behind TLS and a dedicated basic-auth credential.
        </Typography.Paragraph>

        <Space>
          <Switch checked={enabled} loading={busy} onChange={onToggle} />
          <Typography.Text strong>{enabled ? "Exposed" : "Off"}</Typography.Text>
        </Space>

        <div>
          <Typography.Text>Source IP allowlist (optional)</Typography.Text>
          <Space.Compact style={{ width: "100%" }}>
            <Input
              placeholder="e.g. 203.0.113.0/24, 198.51.100.5  (empty = any IP that passes auth)"
              value={cidrs}
              onChange={(e) => setCidrs(e.target.value)}
              disabled={busy}
            />
            <Button onClick={saveAllowlist} loading={busy} disabled={!enabled}>
              Save
            </Button>
          </Space.Compact>
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            Comma/space-separated IPs or CIDRs. Empty = any IP that passes the gateway auth.
          </Typography.Text>
        </div>

        {enabled && (
          <>
            <Alert
              type="warning"
              showIcon
              message={`Live at https://admin.${hostname}/`}
              description="The full mail-server admin is reachable. Restrict by IP and disable when not in use."
            />
            <Button onClick={regenerate} loading={busy}>
              Regenerate gateway credential
            </Button>
          </>
        )}

        <Modal
          open={!!cred}
          title="Gateway credential — shown once"
          onOk={() => setCred(null)}
          onCancel={() => setCred(null)}
          okText="I saved it"
          cancelButtonProps={{ style: { display: "none" } }}
        >
          <Typography.Paragraph>
            Use this at the nginx basic-auth prompt (then Stalwart&apos;s own admin login).
            It is <strong>not stored</strong> and won&apos;t be shown again.
          </Typography.Paragraph>
          <Typography.Paragraph>
            URL: <Typography.Text copyable>{cred?.url}</Typography.Text>
            <br />
            User: <Typography.Text copyable>{cred?.user}</Typography.Text>
            <br />
            Password: <Typography.Text copyable code>{cred?.password}</Typography.Text>
          </Typography.Paragraph>
        </Modal>
      </Space>
    </Card>
  );
}
