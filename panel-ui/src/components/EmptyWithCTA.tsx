// EmptyWithCTA — empty-state placeholder for admin list tables.
// Replaces AntD's generic "No data" with a domain-specific message
// and a primary CTA button so the operator's next step is obvious
// on a fresh install.
//
// Wire it into a Table via `locale={{ emptyText: <EmptyWithCTA ... /> }}`.
// Matches the Docker Apps Installed-tab pattern (Empty + Browse Catalog).
import { Button, Empty } from "antd";
import type { ReactNode } from "react";

interface Props {
  /** Domain-specific empty message, e.g. "No hosting packages yet". */
  description: ReactNode;
  /** CTA button label, e.g. "Create package". Omit to render no button. */
  ctaLabel?: string;
  /** Click handler for the CTA. Required when ctaLabel is set. */
  onCta?: () => void;
  /** Icon to render to the left of the CTA label, if any. */
  ctaIcon?: ReactNode;
}

export const EmptyWithCTA = ({ description, ctaLabel, onCta, ctaIcon }: Props) => (
  <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={description}>
    {ctaLabel && onCta ? (
      <Button type="primary" icon={ctaIcon} onClick={onCta}>
        {ctaLabel}
      </Button>
    ) : null}
  </Empty>
);
