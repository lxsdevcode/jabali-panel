// StatCard — the panel-wide default summary/stat card: a left-aligned tinted
// icon chip, a COLORED label, a large value, and an optional subtitle. The
// accent comes from `iconColor` (label + icon); the chip tint defaults to a
// translucent shade of it (override with `iconBg`). Pass either an `Icon`
// component or an `icon` node. With `to`, the whole card is a hoverable link.
import { Card, Grid, Typography } from "antd";
import type { ComponentType, CSSProperties, ReactNode } from "react";
import { Link } from "react-router";

export interface StatCardProps {
  label: ReactNode;
  value: ReactNode;
  subtitle?: ReactNode;
  /** Accent colour for the label + icon, e.g. "#1677ff". */
  iconColor: string;
  /** Chip background; defaults to a translucent shade of iconColor. */
  iconBg?: string;
  /** Icon as a component (legacy callers)… */
  Icon?: ComponentType<{ style?: CSSProperties }>;
  /** …or as a ready node (dashboards / disk usage). */
  icon?: ReactNode;
  /** When set, the card is a hoverable link to this route. */
  to?: string;
}

// tintFor turns a hex accent into a translucent chip background. Non-hex
// inputs (rgba, named) fall through unchanged.
function tintFor(color: string): string {
  return /^#[0-9a-fA-F]{6}$/.test(color) ? `${color}22` : color;
}

export function StatCard({ label, value, subtitle, iconColor, iconBg, Icon, icon, to }: StatCardProps) {
  const screens = Grid.useBreakpoint();
  // md (4-col rows) is tight — shrink the chip + value a notch.
  const compact = !!screens.md && !screens.lg;
  // Codeberg #7: on xs the 2-up cards are cramped; a fixed 48px icon starved
  // the text column and words fragmented mid-character ("cat alog"). Shrink the
  // chip + gap + value on xs so labels wrap at word boundaries.
  const narrow = !screens.sm;
  const size = narrow ? 36 : compact ? 40 : 48;
  const gap = narrow ? 8 : 12;

  const body = (
    <Card hoverable={!!to} size="small" styles={{ body: { padding: narrow ? 10 : compact ? 12 : 16 } }}>
      <div style={{ display: "flex", alignItems: "center", gap }}>
        <div
          style={{
            flex: "0 0 auto",
            width: size,
            height: size,
            borderRadius: 12,
            background: iconBg ?? tintFor(iconColor),
            color: iconColor,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            fontSize: compact ? 18 : 22,
          }}
        >
          {Icon ? <Icon style={{ fontSize: compact ? 18 : 22 }} /> : icon}
        </div>
        <div style={{ minWidth: 0, overflowWrap: "break-word", wordBreak: "normal" }}>
          <div style={{ color: iconColor, fontSize: 13, fontWeight: 600, marginBottom: 2 }}>{label}</div>
          <div style={{ fontSize: narrow ? 18 : compact ? 20 : 26, fontWeight: 700, lineHeight: 1.1 }}>{value}</div>
          {subtitle != null && (
            <Typography.Text type="secondary" style={{ fontSize: 12, display: "block" }}>
              {subtitle}
            </Typography.Text>
          )}
        </div>
      </div>
    </Card>
  );

  return to ? (
    <Link to={to} style={{ display: "block", color: "inherit" }}>
      {body}
    </Link>
  ) : (
    body
  );
}
