// QuickStartModal — first-login welcome + quick-start guide for the
// tenant (user) shell. Per-user localStorage dismiss; "I'll read later"
// only closes for the session, "Never show again" persists.
import { useEffect, useState, type ComponentType, type CSSProperties } from "react";
import { Button, Modal, Typography } from "antd";
import {
  ApiOutlined,
  AppstoreAddOutlined,
  CheckOutlined,
  ClockCircleOutlined,
  DatabaseOutlined,
  GlobalOutlined,
  MailOutlined,
  QuestionCircleOutlined,
  SaveOutlined,
} from "@icons";
import { Link } from "react-router";

import { useAuth } from "../../auth/AuthContext";

const STORAGE_PREFIX = "jabali:quickstart-user:dismissed:";

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
    title: "Domains",
    desc: "Add your first domain — Jabali provisions the vhost, DNS zone, and SSL automatically.",
    href: "/jabali-panel/domains",
    icon: GlobalOutlined,
    color: "#f59e0b", // amber
  },
  {
    number: 2,
    title: "Mail",
    desc: "Create mailboxes + forwarders on your mail-enabled domains.",
    href: "/jabali-panel/mail/mailboxes",
    icon: MailOutlined,
    color: "#10b981", // green
  },
  {
    number: 3,
    title: "Applications",
    desc: "1-click install WordPress, Joomla, Drupal, and more onto any domain.",
    href: "/jabali-panel/applications",
    icon: AppstoreAddOutlined,
    color: "#3b82f6", // bright blue
  },
  {
    number: 4,
    title: "Databases",
    desc: "Create MariaDB / Postgres databases + per-tenant users.",
    href: "/jabali-panel/databases",
    icon: DatabaseOutlined,
    color: "#2563eb", // blue
  },
  {
    number: 5,
    title: "Backups",
    desc: "Schedule snapshots or trigger an on-demand backup; restore in one click.",
    href: "/jabali-panel/backups",
    icon: SaveOutlined,
    color: "#a855f7", // purple
  },
  {
    number: 6,
    title: "API Tokens",
    desc: "Mint a token for scripting / DDNS — same key works in routers, ddclient, CI.",
    href: "/jabali-panel/api-tokens",
    icon: ApiOutlined,
    color: "#22c55e", // emerald
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
                display: "block",
                padding: "14px 16px",
                overflow: "hidden",
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
                  gap: 8,
                  float: "left",
                  marginRight: 14,
                  marginBottom: 6,
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
          Stuck? Read the docs at{" "}
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
