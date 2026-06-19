// StatCard — the small icon + label + big value + subtitle card used in
// the overview rows on the Docker Apps and Applications pages. Extracted
// so both pages render visually identical stat cards (the layout, sizes,
// and the rounded icon chip all live here, not copied per page).
import { Card, Typography } from "antd";
import type { CSSProperties } from "react";

export interface StatCardProps {
  /** Background of the rounded icon chip, e.g. "rgba(207,19,34,0.12)". */
  iconBg: string;
  /** Foreground colour of the icon, e.g. "#cf1322". */
  iconColor: string;
  Icon: React.ComponentType<{ style?: CSSProperties }>;
  label: string;
  value: number;
  subtitle: React.ReactNode;
}

export function StatCard({ iconBg, iconColor, Icon, label, value, subtitle }: StatCardProps) {
  return (
    <Card size="small">
      <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
        <div
          style={{
            width: 44,
            height: 44,
            borderRadius: 10,
            background: iconBg,
            color: iconColor,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            flexShrink: 0,
          }}
        >
          <Icon style={{ fontSize: 20 }} />
        </div>
        <div style={{ minWidth: 0, flex: 1 }}>
          <Typography.Text type="secondary" style={{ fontSize: 12, display: "block" }}>
            {label}
          </Typography.Text>
          <Typography.Text strong style={{ fontSize: 22, lineHeight: 1.2, display: "block" }}>
            {value}
          </Typography.Text>
          <div style={{ fontSize: 11, marginTop: 2 }}>{subtitle}</div>
        </div>
      </div>
    </Card>
  );
}
