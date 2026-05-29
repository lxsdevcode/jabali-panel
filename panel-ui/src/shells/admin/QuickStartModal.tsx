// QuickStartModal — first-login welcome + quick-start guide for the admin
// shell. Shown until "Never show again" is clicked. State is per-admin in
// localStorage; "I'll read later" closes without persisting (re-shows next
// session). No backend changes — kept simple + offline-safe.
import { useEffect, useState } from "react";
import { Button, List, Modal, Typography } from "antd";
import { Link } from "react-router";

import { useAuth } from "../../auth/AuthContext";

const STORAGE_PREFIX = "jabali:quickstart:dismissed:";

interface Step {
  step: string;
  desc: string;
  href: string;
}

const STEPS: Step[] = [
  {
    step: "1. Server Settings",
    desc: "Set your hostname, primary IPs, and nameservers (ns1/ns2).",
    href: "/jabali-admin/settings",
  },
  {
    step: "2. Hosting Packages",
    desc: "Define disk/bandwidth quota, PHP version, and resource limits for tenants.",
    href: "/jabali-admin/packages",
  },
  {
    step: "3. Users",
    desc: "Create your first tenant account.",
    href: "/jabali-admin/users",
  },
  {
    step: "4. Domains",
    desc: "Add a domain — vhost, DNS zone, and SSL provision automatically.",
    href: "/jabali-admin/domains",
  },
  {
    step: "5. SSL Manager",
    desc: "Let's Encrypt issues automatically. Track + retry from here.",
    href: "/jabali-admin/ssl",
  },
  {
    step: "6. Applications",
    desc: "1-click install WordPress and other apps for any user.",
    href: "/jabali-admin/applications",
  },
];

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
      // localStorage unavailable (private mode / disabled) — skip silently.
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
      title="Welcome to Jabali Panel!"
      open={open}
      onCancel={close}
      width={620}
      destroyOnClose
      footer={[
        <Button key="later" onClick={close}>
          I&apos;ll read later
        </Button>,
        <Button key="never" type="primary" onClick={dismissForever}>
          Never show again
        </Button>,
      ]}
    >
      <Typography.Paragraph>
        Glad to have you here. A short tour of what to do first — click any
        item to jump straight to it:
      </Typography.Paragraph>
      <List
        size="small"
        dataSource={STEPS}
        renderItem={(item) => (
          <List.Item>
            <List.Item.Meta
              title={
                <Link to={item.href} onClick={close}>
                  {item.step}
                </Link>
              }
              description={item.desc}
            />
          </List.Item>
        )}
      />
      <Typography.Paragraph
        type="secondary"
        style={{ marginTop: 16, marginBottom: 0 }}
      >
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
      </Typography.Paragraph>
    </Modal>
  );
}
