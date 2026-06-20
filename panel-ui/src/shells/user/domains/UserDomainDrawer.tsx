// UserDomainDrawer — tenant Add-domain Drawer (replaces the
// /jabali-panel/domains/create page route).
import { Button, Checkbox, Drawer, Form, Grid, Input, Select, Space, message } from "antd";
import { useEffect } from "react";

import { useCreateMutation } from "../../../hooks/useQueries";

type UserDomainCreateInput = {
  name: string;
  mail_provider?: string;
  m365_onmicrosoft?: string;
  google_dkim?: string;
  create_www?: boolean;
  ssl_mode?: string;
};
type DomainCreated = { id: string };

export interface UserDomainDrawerProps {
  open: boolean;
  onClose: () => void;
}

export const UserDomainDrawer = ({ open, onClose }: UserDomainDrawerProps) => {
  const [form] = Form.useForm<UserDomainCreateInput>();
  const mailProvider = Form.useWatch("mail_provider", form) ?? "jabali";
  const screens = Grid.useBreakpoint();
  const isDesktop = screens.lg ?? (typeof window !== "undefined" ? window.innerWidth >= 992 : true);

  const createMutation = useCreateMutation<DomainCreated, UserDomainCreateInput>({
    resource: "domains",
  });

  useEffect(() => {
    if (open) form.resetFields();
  }, [open, form]);

  const handleFinish = async (values: UserDomainCreateInput) => {
    try {
      await createMutation.mutateAsync(values);
      message.success("Domain added");
      onClose();
    } catch (err) {
      message.error(err instanceof Error ? err.message : "Failed to add domain");
    }
  };

  return (
    <Drawer
      title="Add domain"
      open={open}
      onClose={onClose}
      width={isDesktop ? 480 : undefined}
      placement="right"
      destroyOnClose
    >
      <Form<UserDomainCreateInput> form={form} layout="vertical" onFinish={handleFinish}>
        <Form.Item
          label="Domain Name"
          name="name"
          rules={[
            { required: true, message: "Domain name is required" },
            { max: 253, message: "Domain name cannot exceed 253 characters" },
            {
              pattern: /^(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,63}$/,
              message: "Enter a valid domain name (e.g. example.com)",
            },
          ]}
        >
          <Input placeholder="e.g., example.com" />
        </Form.Item>

        <Form.Item
          label="Mail"
          name="mail_provider"
          initialValue="jabali"
          tooltip="Where this domain's email is hosted. 'None' and the external providers skip Jabali's mail DNS records and mail certificate SANs."
        >
          <Select
            options={[
              { value: "jabali", label: "Jabali mail (this server)" },
              { value: "none", label: "No mail" },
              { value: "m365", label: "Microsoft 365" },
              { value: "google", label: "Google Workspace" },
            ]}
          />
        </Form.Item>

        {mailProvider === "m365" && (
          <Form.Item
            label="Microsoft 365 tenant"
            name="m365_onmicrosoft"
            tooltip="Optional. Your <tenant>.onmicrosoft.com — adds the selector1/2 DKIM CNAMEs. MX/SPF/autodiscover are added automatically."
          >
            <Input placeholder="contoso.onmicrosoft.com (optional)" />
          </Form.Item>
        )}

        {mailProvider === "google" && (
          <Form.Item
            label="Google DKIM value"
            name="google_dkim"
            tooltip="Optional. Paste the google._domainkey TXT value from Google Admin. MX/SPF are added automatically."
          >
            <Input.TextArea rows={2} placeholder="v=DKIM1; k=rsa; p=... (optional)" />
          </Form.Item>
        )}

        <Form.Item
          label="TLS certificate"
          name="ssl_mode"
          initialValue="le"
          tooltip="Let's Encrypt issues a free trusted certificate automatically (recommended). Self-signed works without DNS/ACME but browsers warn. None serves over HTTP only."
        >
          <Select
            options={[
              { value: "le", label: "Let's Encrypt (recommended)" },
              { value: "self", label: "Self-signed" },
              { value: "none", label: "None (HTTP only)" },
            ]}
          />
        </Form.Item>

        <Form.Item
          name="create_www"
          valuePropName="checked"
          initialValue={false}
          tooltip="Adds a www CNAME pointing at the domain apex. Off by default — leave unchecked for subdomains or domains that don't serve a www host."
        >
          <Checkbox>Create www record</Checkbox>
        </Form.Item>

        <Form.Item>
          <Space>
            <Button type="primary" htmlType="submit" loading={createMutation.isPending}>
              Add
            </Button>
            <Button onClick={onClose}>Cancel</Button>
          </Space>
        </Form.Item>
      </Form>
    </Drawer>
  );
};
