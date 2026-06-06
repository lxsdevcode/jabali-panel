// DomainCreate — admin form for a new domain.
//
// Intentionally thin: name + user id + optional doc root. The server
// auto-generates doc_root when blank. Post-M21: Form.useForm +
// useCreateMutation, no Refine wrappers.
import { Button, Card, Form, Input, Select, Typography, message } from "antd";
import { useQuery } from "@tanstack/react-query";
import { apiClient } from "../../../apiClient";
import { useNavigate } from "react-router";

import { useCreateMutation } from "../../../hooks/useQueries";

export type DomainCreateInput = {
  name: string;
  user_id: string;
  doc_root?: string;
};

type DomainCreated = { id: string };

export const DomainCreate = () => {
  const navigate = useNavigate();
  const [form] = Form.useForm<DomainCreateInput>();
  const createMutation = useCreateMutation<DomainCreated, DomainCreateInput>({
    resource: "domains",
  });
  const usersQ = useQuery({
    queryKey: ["admin-users-for-domain-create"],
    queryFn: async () => {
      const { data } = await apiClient.get<{ data?: { id: string; email: string; username?: string }[] }>(
        "/users?page_size=500&is_admin=false",
      );
      return data.data ?? [];
    },
  });

  const handleFinish = async (values: DomainCreateInput) => {
    try {
      await createMutation.mutateAsync(values);
      message.success("Domain created");
      navigate("/jabali-admin/domains");
    } catch (err: unknown) {
      const msg =
        err instanceof Error ? err.message : "Failed to create domain";
      message.error(msg);
    }
  };

  return (
    <Card>
      <Typography.Title level={3} style={{ marginTop: 0 }}>
        Create domain
      </Typography.Title>
      <Form<DomainCreateInput>
        form={form}
        layout="vertical"
        onFinish={handleFinish}
      >
        <Form.Item
          label="Name"
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
          label="User"
          name="user_id"
          rules={[{ required: true, message: "User is required" }]}
        >
          <Select
            showSearch
            placeholder="Select a user"
            optionFilterProp="label"
            loading={usersQ.isLoading}
            options={(usersQ.data ?? []).map((u) => ({
              value: u.id,
              label: u.username ? `${u.email} (${u.username})` : u.email,
            }))}
          />
        </Form.Item>

        <Form.Item
          label="Doc Root"
          name="doc_root"
          tooltip="Leave empty for auto-generated path"
        >
          <Input placeholder="auto-generated if empty" />
        </Form.Item>

        <Form.Item>
          <Button
            type="primary"
            htmlType="submit"
            loading={createMutation.isPending}
          >
            Save
          </Button>
        </Form.Item>
      </Form>
    </Card>
  );
};
