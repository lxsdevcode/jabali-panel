// AdminBreadcrumb — shared entity breadcrumb for the admin shell (#483,
// ADR-0152). Presentational only: it maps Crumb[] (built by entityLinks) onto
// AntD's <Breadcrumb>, turning hrefs into react-router <Link>s so navigation is
// SPA-internal, and rendering the optional sibling dropdown via AntD's built-in
// breadcrumb `menu` slot.
import { Breadcrumb } from "antd";
import { Link } from "react-router";

import type { Crumb } from "./entityLinks";

export function AdminBreadcrumb({ items }: { items: Crumb[] }) {
  return (
    <Breadcrumb
      style={{ marginBottom: 16 }}
      items={items.map((c, i) => ({
        // leaf crumb (no href) renders as plain text; others as links.
        title: c.href ? <Link to={c.href}>{c.title}</Link> : c.title,
        // AntD renders a caret + dropdown when `menu.items` is non-empty.
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
