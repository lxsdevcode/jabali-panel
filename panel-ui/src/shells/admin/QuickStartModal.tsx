// QuickStartModal — first-login welcome + quick-start guide for the
// admin shell. Per-admin localStorage dismiss; "I'll read later" only
// closes for the session, "Never show again" persists.
import { useEffect, useState, type ComponentType, type CSSProperties } from "react";
import { Button, Modal, Typography } from "antd";
import {
  AppstoreAddOutlined,
  CheckOutlined,
  ClockCircleOutlined,
  DatabaseOutlined,
  GlobalOutlined,
  InboxOutlined,
  QuestionCircleOutlined,
  SafetyOutlined,
  TeamOutlined,
} from "@icons";
import { Link } from "react-router";

import { useAuth } from "../../auth/AuthContext";

const STORAGE_PREFIX = "jabali:quickstart:dismissed:";

interface Step {
  number: number;
  title: string;
  desc: string;
  href: string;
  icon: ComponentType<{ style?: CSSProperties }>;
  color: string; // accent for icon tile + number badge
}

const STEPS: Step[] = [
  {
    number: 1,
    title: "Server Settings",
    desc: "Set your hostname, primary IPs, and nameservers (ns1/ns2).",
    href: "/jabali-admin/settings",
    icon: DatabaseOutlined,
    color: "#2563eb", // blue
  },
  {
    number: 2,
    title: "Hosting Packages",
    desc: "Define disk/bandwidth quota, PHP version, and resource limits for tenants.",
    href: "/jabali-admin/packages",
    icon: InboxOutlined,
    color: "#10b981", // green
  },
  {
    number: 3,
    title: "Users",
    desc: "Create your first tenant account.",
    href: "/jabali-admin/users",
    icon: TeamOutlined,
    color: "#a855f7", // purple
  },
  {
    number: 4,
    title: "Domains",
    desc: "Add a domain — vhost, DNS zone, and SSL provision automatically.",
    href: "/jabali-admin/domains",
    icon: GlobalOutlined,
    color: "#f59e0b", // amber
  },
  {
    number: 5,
    title: "SSL Manager",
    desc: "Let's Encrypt issues automatically. Track + retry from here.",
    href: "/jabali-admin/ssl",
    icon: SafetyOutlined,
    color: "#22c55e", // emerald
  },
  {
    number: 6,
    title: "Applications",
    desc: "1-click install WordPress and other apps for any user.",
    href: "/jabali-admin/applications",
    icon: AppstoreAddOutlined,
    color: "#3b82f6", // bright blue
  },
];

const hexAlpha = (hex: string, alpha: number) => {
  const n = parseInt(hex.slice(1), 16);
  const r = (n >> 16) & 0xff;
  const g = (n >> 8) & 0xff;
  const b = n & 0xff;
  return `rgba(${r}, ${g}, ${b}, ${alpha})`;
};

export function QuickStartModal() {
  const { user } = useAuth();
  const [open, setOpen] = useState(false);

  useEffect(() => {
    if (!user?.id) return;
    try {
      if (!localStorage.getItem(STORAGE_PREFIX + user.id)) {
        setOpen(true);
      }
    } catch {
      // localStorage unavailable — skip silently.
    }
  }, [user?.id]);

  const close = () => setOpen(false);

  const dismissForever = () => {
    if (user?.id) {
      try {
        localStorage.setItem(STORAGE_PREFIX + user.id, "1");
      } catch {
        // ignore
      }
    }
    setOpen(false);
  };

  return (
    <Modal
      title={
        <div style={{ display: "flex", alignItems: "center", gap: 14 }}>
          <div
            style={{
              width: 44,
              height: 44,
              borderRadius: 12,
              background: hexAlpha("#3b82f6", 0.18),
              color: "#3b82f6",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              fontSize: 22,
              flexShrink: 0,
            }}
          >
            <span role="img" aria-label="jabali">
              🐃
            </span>
          </div>
          <Typography.Title level={3} style={{ margin: 0 }}>
            Welcome to Jabali Panel!{" "}
            <span role="img" aria-label="wave">
              👋
            </span>
          </Typography.Title>
        </div>
      }
      open={open}
      onCancel={close}
      width={960}
      style={{ maxWidth: "calc(100vw - 32px)" }}
      styles={{ wrapper: { padding: "16px" } }}
      destroyOnClose
      centered
      footer={[
        <Button key="later" icon={<ClockCircleOutlined />} onClick={close}>
          I&apos;ll read later
        </Button>,
        <Button
          key="never"
          type="primary"
          icon={<CheckOutlined />}
          onClick={dismissForever}
        >
          Never show again
        </Button>,
      ]}
    >
      <Typography.Paragraph type="secondary" style={{ marginBottom: 20 }}>
        Glad to have you here. A short tour of what to do first — click any
        item to jump straight to it:
      </Typography.Paragraph>

      <div
        style={{
          display: "grid",
          gridTemplateColumns: "repeat(auto-fit, minmax(260px, 1fr))",
          gap: 12,
        }}
      >
        {STEPS.map((step) => {
          const Icon = step.icon;
          return (
            <Link
              key={step.number}
              to={step.href}
              onClick={close}
              style={{
                display: "flex",
                flexDirection: "column",
                alignItems: "stretch",
                gap: 12,
                padding: "14px 16px",
                borderRadius: 12,
                border: "1px solid rgba(255,255,255,0.08)",
                background: "rgba(255,255,255,0.02)",
                color: "inherit",
                textDecoration: "none",
                transition: "background 0.15s, border-color 0.15s",
              }}
              onMouseEnter={(e) => {
                (e.currentTarget as HTMLAnchorElement).style.background =
                  "rgba(255,255,255,0.05)";
                (e.currentTarget as HTMLAnchorElement).style.borderColor =
                  "rgba(255,255,255,0.18)";
              }}
              onMouseLeave={(e) => {
                (e.currentTarget as HTMLAnchorElement).style.background =
                  "rgba(255,255,255,0.02)";
                (e.currentTarget as HTMLAnchorElement).style.borderColor =
                  "rgba(255,255,255,0.08)";
              }}
            >
              <div
                style={{
                  display: "flex",
                  flexDirection: "row",
                  alignItems: "center",
                  gap: 12,
                }}
              >
              <div
                style={{
                  width: 56,
                  height: 56,
                  borderRadius: 14,
                  background: hexAlpha(step.color, 0.18),
                  color: step.color,
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  fontSize: 28,
                  flexShrink: 0,
                }}
              >
                <Icon style={{ fontSize: 28 }} />
              </div>
              <div
                style={{
                  width: 28,
                  height: 28,
                  borderRadius: "50%",
                  background: step.color,
                  color: "white",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  fontSize: 13,
                  fontWeight: 600,
                  flexShrink: 0,
                }}
              >
                {step.number}
              </div>
              </div>
              <div style={{ flex: 1, minWidth: 0 }}>
                <Typography.Text strong style={{ fontSize: 16, display: "block" }}>
                  {step.title}
                </Typography.Text>
                <Typography.Text type="secondary" style={{ fontSize: 13 }}>
                  {step.desc}
                </Typography.Text>
              </div>
              
            </Link>
          );
        })}
      </div>

      <div
        style={{
          marginTop: 20,
          padding: "16px 18px",
          borderRadius: 12,
          background: hexAlpha("#3b82f6", 0.08),
          border: "1px solid " + hexAlpha("#3b82f6", 0.25),
          display: "flex",
          alignItems: "center",
          gap: 14,
        }}
      >
        <QuestionCircleOutlined
          style={{ fontSize: 24, color: "#3b82f6", flexShrink: 0 }}
        />
        <Typography.Text style={{ flex: 1 }}>
          Stuck? Use{" "}
          <Link to="/jabali-admin/support" onClick={close}>
            Support
          </Link>{" "}
          or read the docs at{" "}
          <a
            href="https://jabali-panel.com"
            target="_blank"
            rel="noopener noreferrer"
          >
            jabali-panel.com
          </a>
          .
        </Typography.Text>
        <span
          role="img"
          aria-label="docs"
          style={{ fontSize: 28, flexShrink: 0 }}
        >
          📖
        </span>
      </div>
    </Modal>
  );
}
