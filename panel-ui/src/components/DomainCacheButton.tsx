// DomainCacheButton — a Modal wrapper around DomainCacheSection so the nginx
// page-cache opt-in (toggle + purge) is reachable from the domain row's "More"
// menu in both the admin and user shells, not only via Edit → Caching.
import { Modal } from "antd";

import { DomainCacheSection } from "./DomainCacheSection";

interface DomainCacheButtonProps {
  domainId: string;
  domainName?: string;
  open: boolean;
  onClose: () => void;
}

export const DomainCacheButton = ({
  domainId,
  domainName,
  open,
  onClose,
}: DomainCacheButtonProps) => (
  <Modal
    title={domainName ? `Caching — ${domainName}` : "Caching"}
    open={open}
    onCancel={onClose}
    footer={null}
    destroyOnClose
    width={640}
  >
    <DomainCacheSection domainId={domainId} />
  </Modal>
);
