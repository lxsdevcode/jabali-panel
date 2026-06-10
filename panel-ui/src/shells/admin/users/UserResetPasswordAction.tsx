// UserResetPasswordAction — admin sets a fresh password for a user and
// reveals it once. Under username-only login (M54) email is a plain trait,
// so Kratos has no self-service recovery; this admin-set flow is the only
// password reset. Mirrors UserReset2FAAction's controlled-modal shape; the
// parent (UserList RowActions) drives `open`.
import { useState } from "react";
import { Modal, Typography, message } from "antd";

import { apiClient } from "../../../apiClient";

interface Props {
  userId: string;
  userEmail: string;
  open: boolean;
  onClose: () => void;
}

export const UserResetPasswordAction = ({ userId, userEmail, open, onClose }: Props) => {
  const [isLoading, setIsLoading] = useState(false);
  const [tempPassword, setTempPassword] = useState<string | null>(null);

  const handleReset = async () => {
    setIsLoading(true);
    try {
      const res = await apiClient.post<{ password: string }>(
        `/admin/users/${encodeURIComponent(userId)}/password/reset`,
      );
      setTempPassword(res.data.password);
      message.success(`Password reset for "${userEmail}"`);
    } catch (err: unknown) {
      message.error(err instanceof Error ? err.message : "Failed to reset password");
    } finally {
      setIsLoading(false);
    }
  };

  const close = () => {
    setTempPassword(null);
    onClose();
  };

  if (tempPassword !== null) {
    return (
      <Modal
        title="New password — shown once"
        open={open}
        onCancel={close}
        onOk={close}
        cancelButtonProps={{ style: { display: "none" } }}
        okText="Done"
      >
        <p>
          Hand this temporary password to <strong>{userEmail}</strong>. It is shown
          only once. They sign in with it and should change it from their profile.
        </p>
        <Typography.Paragraph copyable strong style={{ fontSize: 16 }}>
          {tempPassword}
        </Typography.Paragraph>
      </Modal>
    );
  }

  return (
    <Modal
      title="Reset password?"
      open={open}
      onCancel={close}
      onOk={handleReset}
      confirmLoading={isLoading}
      okText="Reset"
      okButtonProps={{ danger: true }}
    >
      <p>
        Sets a new temporary password for <strong>{userEmail}</strong> and shows it
        once. Their current password stops working immediately.
      </p>
      <p>There is no self-service recovery under username login — this is the reset.</p>
    </Modal>
  );
};
