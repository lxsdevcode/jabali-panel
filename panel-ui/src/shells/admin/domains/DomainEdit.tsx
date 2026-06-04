// DomainEdit — admin domain editor. Tabs split General / SSL / Caching /
// Network / Security / Email so the page stops being a 1500px scroll. Each
// tab owns one section component; General hosts the basic Form
// (name/doc_root/enabled/custom directives + Save).
import { useEffect } from "react";
import {
  Alert,
  Button,
  Card,
  Form,
  Space,
  Spin,
  Switch,
  Tabs,
  Typography,
  message,
} from "antd";
import { CheckOutlined, CloseOutlined } from "@icons";
import { useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate, useParams } from "react-router";

import { useOneQuery, useUpdateMutation } from "../../../hooks/useQueries";
import type { Domain } from "./DomainList";
import { DomainEmailSection } from "./DomainEmailSection";
import { DomainIPACLSection } from "./DomainIPACLSection";
import { DomainDirectoryPrivacySection } from "./DomainDirectoryPrivacySection";
import { DomainListenIPSection } from "./DomainListenIPSection";
import { DomainMailboxesSection } from "./DomainMailboxesSection";
import { DomainNginxSection } from "../../DomainSettingsButton";
import { DomainSSLSection } from "./DomainSSLSection";
import { DomainSkipAutoSANToggle } from "./DomainSkipAutoSANToggle";
import { DomainCacheSection } from "./DomainCacheSection";
import { DomainMTASTSSection } from "./DomainMTASTSSection";
import { DomainDeliverabilitySection } from "./DomainDeliverabilitySection";

export type DomainEditInput = {
  is_enabled?: boolean;
};

export const DomainEdit = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const qc = useQueryClient();
  const [form] = Form.useForm<DomainEditInput>();

  const { data: domain, isLoading } = useOneQuery<Domain>({
    resource: "domains",
    id,
  });
  const updateMutation = useUpdateMutation<Domain, DomainEditInput>({
    resource: "domains",
  });

  useEffect(() => {
    if (domain) {
      form.setFieldsValue({
        is_enabled: domain.is_enabled,
      });
    }
  }, [domain, form]);

  const handleFinish = async (values: DomainEditInput) => {
    if (!id) return;
    try {
      await updateMutation.mutateAsync({ id, input: values });
      message.success("Domain updated");
      navigate("/jabali-admin/domains");
    } catch (err: unknown) {
      const msg =
        err instanceof Error ? err.message : "Failed to update domain";
      message.error(msg);
    }
  };

  if (isLoading && !domain) {
    return (
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          minHeight: 240,
        }}
      >
        <Spin />
      </div>
    );
  }

  if (!domain) {
    return null;
  }

  const generalTab = (
    <Form<DomainEditInput> form={form} layout="vertical" onFinish={handleFinish}>
      <Form.Item label="Name">
        <Typography.Text>{domain.name}</Typography.Text>
      </Form.Item>

      <Form.Item label="Doc Root">
        <Typography.Text>{domain.doc_root || "auto-generated"}</Typography.Text>
      </Form.Item>

      <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 24 }}>
        <Form.Item name="is_enabled" valuePropName="checked" noStyle>
          <Switch
            checkedChildren={<CheckOutlined />}
            unCheckedChildren={<CloseOutlined />}
          />
        </Form.Item>
        <Typography.Text>Enabled</Typography.Text>
      </div>

      <Form.Item>
        <Button type="primary" htmlType="submit" loading={updateMutation.isPending}>
          Save
        </Button>
      </Form.Item>
    </Form>
  );

  // Panel-hostname row (ADR-0048) is mail-only: no HTTP vhost, no
  // per-tenant SSL cert (the singleton panel cert covers it via SAN),
  // no docroot. Hide tabs that don't apply; render the rest with an
  // explanatory banner so the operator knows where to manage SSL.
  const isPanelPrimary = !!domain.is_panel_primary;

  const sslTab = {
    key: "ssl",
    label: "SSL / HTTPS",
    children: (
      <Space direction="vertical" style={{ width: "100%" }} size="large">
        <DomainSSLSection
          domainId={domain.id}
          sslEnabled={!!domain.ssl_enabled}
          onToggled={() =>
            qc.invalidateQueries({ queryKey: ["one", "domains", id] })
          }
        />
        <DomainSkipAutoSANToggle domainId={domain.id} />
      </Space>
    ),
  };

  const cacheTab = {
    key: "cache",
    label: "Caching",
    children: <DomainCacheSection domainId={domain.id} />,
  };

  const networkTab = {
    key: "network",
    label: "Network",
    children: (
      <DomainListenIPSection
        domainId={domain.id}
        listenIPv4ID={domain.listen_ipv4_id ?? null}
        listenIPv6ID={domain.listen_ipv6_id ?? null}
        listenIPv4={domain.listen_ipv4 ?? null}
        listenIPv6={domain.listen_ipv6 ?? null}
      />
    ),
  };

  const nginxTab = {
    key: "nginx",
    label: "Nginx",
    children: <DomainNginxSection domain={domain} />,
  };

  const securityTab = {
    key: "security",
    label: "Security",
    children: (
      <Space direction="vertical" style={{ width: "100%" }} size="large">
        <div>
          <Typography.Title level={5} style={{ marginTop: 0 }}>
            IP Allow / Deny
          </Typography.Title>
          <DomainIPACLSection domainId={domain.id} />
        </div>
        <div>
          <Typography.Title level={5}>Directory Privacy</Typography.Title>
          <DomainDirectoryPrivacySection
            domainId={domain.id}
            domainName={domain.name}
          />
        </div>
      </Space>
    ),
  };

  const emailTab = {
    key: "email",
    label: "Email",
    children: (
      <Space direction="vertical" style={{ width: "100%" }} size="large">
        <div>
          <Typography.Title level={5} style={{ marginTop: 0 }}>
            Incoming + Outgoing
          </Typography.Title>
          <DomainEmailSection domainId={domain.id} />
        </div>
        <div>
          <Typography.Title level={5}>Mailboxes</Typography.Title>
          <DomainMailboxesSection domainId={domain.id} />
        </div>
        <div>
          <Typography.Title level={5}>MTA-STS</Typography.Title>
          <DomainMTASTSSection domainId={domain.id} />
        </div>
        <div>
          <Typography.Title level={5}>Deliverability</Typography.Title>
          <DomainDeliverabilitySection domainName={domain.name} />
        </div>
      </Space>
    ),
  };

  const tabs = isPanelPrimary
    ? [{ key: "general", label: "General", children: generalTab }, emailTab]
    : [
        { key: "general", label: "General", children: generalTab },
        sslTab,
        cacheTab,
        networkTab,
        nginxTab,
        securityTab,
        emailTab,
      ];

  return (
    <Space direction="vertical" size="middle" style={{ width: "100%" }}>
      <Card>
        <Typography.Title level={3} style={{ marginTop: 0 }}>
          Edit domain — {domain.name}
        </Typography.Title>
        {isPanelPrimary && (
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
            message="Panel hostname domain (mail-only)"
            description={
              <>
                This row backs the panel hostname's mail service (DKIM,
                Stalwart, MTA-STS). It has no HTTP vhost, docroot, or
                per-domain SSL cert. Manage the panel's TLS cert at{" "}
                <Link to="/jabali-admin/settings">
                  Server Settings &rarr; Panel SSL
                </Link>
                . Caching, listen IPs, IP ACLs, and Directory Privacy
                don't apply here and are hidden.
              </>
            }
          />
        )}
        <Tabs items={tabs} defaultActiveKey="general" destroyInactiveTabPane />
      </Card>
    </Space>
  );
};
