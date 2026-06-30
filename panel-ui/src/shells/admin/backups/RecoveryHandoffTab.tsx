// Gitea #543 — Disaster-recovery handoff. The GUI supports normal account
// restore (the Restore row action); the advanced flows live in the CLI:
// staging-only account restore, deleted-user account restore, and bare-metal
// system restore. This tab makes those discoverable BEFORE an incident, with
// copy-able command templates and the encryption-key prerequisite, instead of
// leaving operators to find them mid-disaster.
import { Alert, Space, Table, Typography } from "antd";
import { LifeBuoyOutlined, KeyOutlined } from "@icons";

const { Paragraph, Text, Title } = Typography;

interface FlowRow {
  key: string;
  flow: string;
  where: "GUI" | "CLI";
  note: string;
}

const FLOWS: FlowRow[] = [
  { key: "account", flow: "Restore an account from a successful backup", where: "GUI", note: "Backups tab → row → Restore. Applies the snapshot for an existing panel user." },
  { key: "stage", flow: "Staging-only account restore (materialize, do not apply)", where: "CLI", note: "Recon before a real restore — extracts to staging so you can inspect first." },
  { key: "deleted", flow: "Restore an account whose panel user row is gone", where: "CLI", note: "Disaster recovery — re-creates the system account from the original identifiers." },
  { key: "system", flow: "System restore (bare-metal / full host)", where: "CLI", note: "Overwrites the running host. CLI-only by design; never a one-click browser action." },
];

// Command templates. Placeholders in <angle-brackets>; the operator fills real
// values (pick the snapshot id from `jabali backup account-restore --force`
// interactive mode, or the Backups tab). Kept verbatim in sync with the CLI
// help in account_restore_cmd.go / system_restore_cmd.go.
const CMD_STAGE =
  "jabali backup account-restore --user <username> --destination <destination> \\\n" +
  "    --snapshot <snapshot-id> --apply=false --force";

const CMD_DELETED =
  "jabali backup account-restore \\\n" +
  "    --target-user-id <original-ulid> --target-username <username> \\\n" +
  "    --destination <destination> --snapshot <snapshot-id> --force";

const CMD_INTERACTIVE = "jabali backup account-restore --force";

const CMD_SYSTEM =
  "# On a fresh OS install (bare-metal recovery):\n" +
  "bash <(curl -fsSL https://<panel-host>/install.sh) --restore-from=<repo-url>\n" +
  "jabali system restore --snapshot=latest --force\n\n" +
  "# Or interactively (prompts for repo URL, restic password, snapshot, stages):\n" +
  "jabali system restore --interactive";

const CommandBlock = ({ label, cmd }: { label: string; cmd: string }) => (
  <div style={{ marginBottom: 16 }}>
    <Text strong>{label}</Text>
    <Paragraph
      copyable={{ text: cmd }}
      style={{
        marginTop: 6,
        marginBottom: 0,
        background: "var(--ant-color-fill-tertiary, #f5f5f5)",
        padding: "8px 12px",
        borderRadius: 6,
        fontFamily: "monospace",
        whiteSpace: "pre-wrap",
      }}
    >
      {cmd}
    </Paragraph>
  </div>
);

export const RecoveryHandoffTab = () => {
  return (
    <Space direction="vertical" size="large" style={{ width: "100%" }}>
      <Alert
        type="info"
        showIcon
        icon={<LifeBuoyOutlined />}
        message="Restore: what the panel does vs. what the CLI does"
        description="Normal account restore runs from the Backups tab. The advanced and disaster-recovery flows below run from the root shell on the host — they are intentionally CLI-only so they work even when the panel itself is down."
      />

      <Table<FlowRow>
        size="small"
        pagination={false}
        dataSource={FLOWS}
        columns={[
          { title: "Restore flow", dataIndex: "flow", key: "flow" },
          {
            title: "Where",
            dataIndex: "where",
            key: "where",
            width: 90,
            render: (w: FlowRow["where"]) => (
              <Text strong style={{ color: w === "GUI" ? "var(--ant-color-success, #389e0d)" : "var(--ant-color-warning, #d46b08)" }}>
                {w}
              </Text>
            ),
          },
          { title: "Notes", dataIndex: "note", key: "note", responsive: ["md"] },
        ]}
      />

      <div>
        <Title level={5}>Advanced account restore (CLI)</Title>
        <CommandBlock label="Interactive — pick destination, snapshot, and user from real lists" cmd={CMD_INTERACTIVE} />
        <CommandBlock label="Staging-only (materialize to staging, do not apply)" cmd={CMD_STAGE} />
        <CommandBlock label="Deleted-user restore (panel user row is gone)" cmd={CMD_DELETED} />
      </div>

      <div>
        <Title level={5}>System restore (CLI-only)</Title>
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 12 }}
          message="System restore is not available in the browser"
          description="Bare-metal / full-host restore overwrites the running host and is run from the root shell, never as a one-click browser action. Use the template below on a clean OS install."
        />
        <CommandBlock label="Bare-metal recovery sequence" cmd={CMD_SYSTEM} />
      </div>

      <Alert
        type="warning"
        showIcon
        icon={<KeyOutlined />}
        message="You need the backup encryption key to restore"
        description="Every restore flow above requires the restic master encryption key for the repository. Reveal and stash it now from the Encryption key tab — without it, no snapshot can be decrypted and recovery is impossible."
      />
    </Space>
  );
};
