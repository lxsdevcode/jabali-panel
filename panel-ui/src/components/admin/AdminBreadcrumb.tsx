// AdminBreadcrumb — shared entity breadcrumb for the admin shell (#483,
// ADR-0152). Presentational only.
//
// Wide (≥lg): maps Crumb[] onto AntD's <Breadcrumb>, turning hrefs into
// react-router <Link>s and rendering the optional sibling dropdown via AntD's
// breadcrumb `menu` slot.
//
// Tablet/mobile (<lg): the full trail won't fit, so it collapses to a single
// "current page ▾" dropdown whose menu lists every page in the trail (plus any
// sibling resources) — one tap to jump anywhere. (#483 mobile follow-up.)
import { DownOutlined } from "@icons";
import { Breadcrumb, Button, Dropdown, Grid } from "antd";
import { Link } from "react-router";

import { type Crumb, crumbsToMenuItems } from "./entityLinks";

export function AdminBreadcrumb({ items }: { items: Crumb[] }) {
  const screens = Grid.useBreakpoint();
  // Only collapse when we KNOW the viewport is narrow. On the first paint /
  // SSR / jsdom, screens.lg is undefined — treat that as wide so the full
  // trail (and the unit tests) render unchanged.
  const narrow = screens.lg === false;

  if (narrow && items.length > 1) {
    const menuItems = crumbsToMenuItems(items).map((m) => ({
      key: m.key,
      label: <Link to={m.href}>{m.label}</Link>,
    }));
    const current = items[items.length - 1].title;
    return (
      <div style={{ marginBottom: 16 }}>
        <Dropdown menu={{ items: menuItems }} trigger={["click"]} disabled={menuItems.length === 0}>
          <Button>
            {current} <DownOutlined />
          </Button>
        </Dropdown>
      </div>
    );
  }

  return (
    <Breadcrumb
      style={{ marginBottom: 16 }}
      items={items.map((c, i) => ({
        title: c.href ? <Link to={c.href}>{c.title}</Link> : c.title,
        menu:
          c.menu && c.menu.length > 0
            ? {
                items: c.menu.map((m) => ({
                  key: m.key,
                  label: <Link to={m.href}>{m.label}</Link>,
                })),
              }
            : undefined,
        key: i,
      }))}
    />
  );
}
