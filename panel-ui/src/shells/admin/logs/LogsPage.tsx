// LogsPage — consolidated Logs & Statistics for admins. One tabbed home
// for every log surface that used to be scattered:
//   Domains  — per-domain web access/error/GoAccess streams (was this page)
//   Audit    — the admin audit log (moved off its own sidebar entry)
//   Backups  — backup job logs (reused from the Backups page)
//   Mail     — the mail trace log (admin sees all domains)
import { useTabParam } from "../../../hooks/useTabParam";
import { Tabs, Typography } from "antd";

import { DomainLogsTab } from "./DomainLogsTab";
import { AdminAuditList } from "../audit/AdminAuditList";
import { BackupLogsTab } from "../backups/BackupLogsTab";
import { LogsTab as MailLogsTab } from "../../user/mail/tabs/LogsTab";
import { UpdatesLogTab } from "./UpdatesLogTab";

const { Title } = Typography;

export const LogsPage = () => {
  const [activeTab, setActiveTab] = useTabParam<string>("domains");

  return (
    <div>
      <Title level={2} style={{ marginTop: 0, marginBottom: 16 }}>
        Logs & Statistics
      </Title>

      <Tabs
        activeKey={activeTab}
        onChange={setActiveTab}
        items={[
          { key: "domains", label: "Domains", children: <DomainLogsTab /> },
          { key: "audit", label: "Audit Log", children: <AdminAuditList /> },
          { key: "backups", label: "Backups", children: <BackupLogsTab /> },
          { key: "updates", label: "Updates", children: <UpdatesLogTab /> },
          { key: "mail", label: "Mail", children: <MailLogsTab /> },
        ]}
      />
    </div>
  );
};
