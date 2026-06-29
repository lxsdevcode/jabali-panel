// StandardActionFooter — shared footer action row for drawers and modals
// (Gitea #534). Gives every drawer/modal a consistent, visually-separated
// action area instead of buttons glued to a form/help card inside the body.
//
// Layout: [extra ............ Cancel  Primary], right-aligned primary, with an
// optional left-aligned `extra` slot (e.g. "Send test", a destructive action).
//
// For form drawers the primary button submits an external <Form id={formId}>
// via the HTML `form` attribute, so the actions can live in the Drawer/Modal
// footer (outside the <Form>) yet still trigger validation + onFinish.
import type { ReactNode } from "react";
import { Button, Flex, Space } from "antd";

export interface StandardActionFooterProps {
  /** Primary button label (e.g. "Save", "Create"). */
  primaryText: string;
  /** Associate the primary submit button with a <Form id={formId}>. */
  formId?: string;
  /** When no formId, handle the click directly. */
  onPrimary?: () => void;
  primaryLoading?: boolean;
  primaryDisabled?: boolean;
  primaryDanger?: boolean;
  /** Secondary (cancel) action; omit to hide. */
  onCancel?: () => void;
  cancelText?: string;
  /** Left-aligned extra content (secondary/destructive action, e.g. test). */
  extra?: ReactNode;
}

export function StandardActionFooter({
  primaryText,
  formId,
  onPrimary,
  primaryLoading,
  primaryDisabled,
  primaryDanger,
  onCancel,
  cancelText = "Cancel",
  extra,
}: StandardActionFooterProps) {
  return (
    <Flex align="center" justify="space-between" gap="small" wrap="wrap">
      <div>{extra}</div>
      <Space wrap>
        {onCancel ? <Button onClick={onCancel}>{cancelText}</Button> : null}
        <Button
          type="primary"
          danger={primaryDanger}
          loading={primaryLoading}
          disabled={primaryDisabled}
          {...(formId ? { htmlType: "submit", form: formId } : { onClick: onPrimary })}
        >
          {primaryText}
        </Button>
      </Space>
    </Flex>
  );
}

// StandardModalFooter is the same action row for antd Modal footers — alias so
// call sites read intent-fully and we can diverge later if a modal needs it.
export const StandardModalFooter = StandardActionFooter;
export const StandardDrawerFooter = StandardActionFooter;
