// LookAndFeelCard — panel-wide appearance: font size + operator colors. Stored
// as a server_settings singleton and applied for ALL users via the app's
// ConfigProvider (useBranding -> useMuiTheme). Empty color = built-in default.
import { useEffect, useState } from "react";

import {
  Button,
  Card,
  ColorPicker,
  Form,
  Segmented,
  Space,
  message,
} from "antd";
import { useQueryClient } from "@tanstack/react-query";

import { SaveOutlined } from "@icons";

import { apiClient } from "../../../apiClient";

type Shape = {
  panel_font_size: "small" | "medium" | "large";
  panel_primary_color: string;
  panel_accent_color: string;
};

// ColorPicker stores an antd Color object on change but a plain hex string on
// initial load — normalize either (or empty) to a hex string for the API.
const toHex = (c: unknown): string => {
  if (!c) return "";
  if (typeof c === "string") return c;
  if (
    typeof c === "object" &&
    c !== null &&
    "toHexString" in c &&
    typeof (c as { toHexString: unknown }).toHexString === "function"
  ) {
    return (c as { toHexString: () => string }).toHexString();
  }
  return "";
};

export const LookAndFeelCard = () => {
  const qc = useQueryClient();
  const [form] = Form.useForm<Shape>();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const resp = await apiClient.get<{
          panel_font_size?: string;
          panel_primary_color?: string;
          panel_accent_color?: string;
        }>("/admin/settings");
        if (cancelled) return;
        form.setFieldsValue({
          panel_font_size:
            (resp.data.panel_font_size as Shape["panel_font_size"]) ?? "medium",
          panel_primary_color: resp.data.panel_primary_color ?? "",
          panel_accent_color: resp.data.panel_accent_color ?? "",
        });
      } catch (err) {
        message.error(err instanceof Error ? err.message : "Load failed");
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [form]);

  const handleSubmit = async (values: Shape) => {
    setSaving(true);
    try {
      await apiClient.patch("/admin/settings", {
        panel_font_size: values.panel_font_size ?? "medium",
        panel_primary_color: toHex(values.panel_primary_color),
        panel_accent_color: toHex(values.panel_accent_color),
      });
      qc.invalidateQueries({ queryKey: ["branding", "public"] });
      message.success("Look and feel saved");
    } catch (err) {
      message.error(err instanceof Error ? err.message : "Save failed");
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card title="Look and feel" style={{ marginBottom: 16 }}>
      <Form
        form={form}
        layout="vertical"
        onFinish={handleSubmit}
        disabled={loading}
      >
        <Form.Item
          label="Panel font size"
          name="panel_font_size"
          extra="Applies to the whole panel for all users. Medium is the default."
        >
          <Segmented
            options={[
              { label: "Small", value: "small" },
              { label: "Medium", value: "medium" },
              { label: "Large", value: "large" },
            ]}
          />
        </Form.Item>

        <Form.Item
          label="Primary color"
          name="panel_primary_color"
          getValueProps={(v) => ({ value: v || undefined })}
          extra="Buttons, links, primary actions and selected toggles. Clear = default blue."
        >
          <ColorPicker allowClear format="hex" />
        </Form.Item>

        <Form.Item
          label="Accent color"
          name="panel_accent_color"
          getValueProps={(v) => ({ value: v || undefined })}
          extra="Selected sidebar row and active tab. Clear = default."
        >
          <ColorPicker allowClear format="hex" />
        </Form.Item>

        <Space>
          <Button
            type="primary"
            icon={<SaveOutlined />}
            loading={saving}
            htmlType="submit"
          >
            Save
          </Button>
        </Space>
      </Form>
    </Card>
  );
};
