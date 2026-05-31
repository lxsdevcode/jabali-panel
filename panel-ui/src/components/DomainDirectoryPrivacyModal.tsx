// DomainDirectoryPrivacyModal — wraps DomainDirectoryPrivacySection in a
// Modal so the user-shell domain row can launch it from a dropdown menu
// item. Admin shell uses the section directly inside the Security tab;
// this is the lightweight equivalent for tenants who don't have a
// per-domain edit page.
import { Modal } from "antd";

import { DomainDirectoryPrivacySection } from "../shells/admin/domains/DomainDirectoryPrivacySection";

type Props = {
  open: boolean;
  domainId: string;
  domainName?: string;
  onClose: () => void;
};

export const DomainDirectoryPrivacyModal = ({
  open,
  domainId,
  domainName,
  onClose,
}: Props) => (
  <Modal
    open={open}
    title={domainName ? `Directory Privacy — ${domainName}` : "Directory Privacy"}
    onCancel={onClose}
    width={820}
    destroyOnClose
    footer={null}
  >
    <DomainDirectoryPrivacySection
      domainId={domainId}
      domainName={domainName}
    />
  </Modal>
);
