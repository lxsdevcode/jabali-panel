// SystemUpdatesPage — admin self-update page (M29).
//
// Two stacked cards:
//   1. Jabali Panel: Check for updates → if behind, "Update Jabali panel"
//      button kicks off `system.update_run`. Status + log tail polled
//      every 2 s while the unit is active.
//   2. System Packages: Check for updates → runs apt-get update + parses
//      apt list --upgradable. Renders a 3-col table; "Apply updates"
//      button starts dist-upgrade as a transient unit.
import { useState } from "react";
import {
  Alert,
  Button,
  Card,
  Checkbox,
  Col,
  Empty,
  Row,
  Space,
  Statistic,
  Switch,
  Table,
  Tag,
  TimePicker,
  Timeline,
  Tooltip,
  Typography,
  message,
} from "antd";
import dayjs from "dayjs";

import {
  ClockCircleOutlined,
  DownloadOutlined,
  FileTextOutlined,
  LoadingOutlined,
  ReloadOutlined,
  SafetyOutlined,
  SettingOutlined,
  SyncOutlined,
} from "@icons";

import { JobLogTail } from "../../../components/JobLogTail";
import {
  useAptCheck,
  useAptRun,
  useAptStatus,
  useAptStop,
  useAutoupdateConfig,
  useChangelog,
  useJabaliCheck,
  useJabaliRun,
  useJabaliStatus,
  useJabaliStop,
  useUpdateAutoupdate,
  useUpdateHistory,
  useUpdateState,
  type AptPackage,
  type AutoupdateConfig,
  type UpdateHistoryRow,
} from "../../../hooks/useSystemUpdates";

export const SystemUpdatesPage = () => (
  <div>
    <Typography.Title level={3} style={{ marginTop: 0, marginBottom: 16 }}>
      <DownloadOutlined /> Updates
    </Typography.Title>
    <Space direction="vertical" size={16} style={{ width: "100%" }}>
      <UpdateStatsRow />
      <JabaliUpdateCard />
      <AptUpdateCard />
      <AutomaticUpdatesCard />
      <RecentHistoryCard />
      <ChangelogCard />
    </Space>
  </div>
);

// UpdateStatsRow — three at-a-glance cards from the persisted update_state
// snapshot (no agent call needed on page load).
function UpdateStatsRow() {
  const { data } = useUpdateState();
  const security = data?.apt_security ?? 0;
  return (
    <Row gutter={[16, 16]}>
      <Col xs={24} sm={8}>
        <Card>
          <Statistic
            title="Panel commits behind"
            value={data?.jabali_behind ?? 0}
            prefix={<DownloadOutlined />}
            valueStyle={{ color: (data?.jabali_behind ?? 0) > 0 ? "#d46b08" : undefined }}
          />
        </Card>
      </Col>
      <Col xs={24} sm={8}>
        <Card>
          <Statistic
            title="OS packages upgradable"
            value={data?.apt_total ?? 0}
            prefix={<SyncOutlined />}
          />
        </Card>
      </Col>
      <Col xs={24} sm={8}>
        <Card>
          <Statistic
            title="Security updates"
            value={security}
            prefix={<SafetyOutlined />}
            valueStyle={{ color: security > 0 ? "#cf1322" : "#3f8600" }}
          />
        </Card>
      </Col>
    </Row>
  );
}

function JabaliUpdateCard() {
  const [since, setSince] = useState<string | null>(null);
  const check = useJabaliCheck();
  const run = useJabaliRun();
  const stop = useJabaliStop();
  const status = useJabaliStatus(since);

  const result = check.data;
  const running =
    status.data?.status === "active" || status.data?.status === "activating";
  const finished =
    since !== null &&
    status.data !== undefined &&
    !running &&
    status.data.exit_code !== undefined;
  const succeeded = finished && status.data?.exit_code === 0;

  const onCheck = async () => {
    try {
      await check.mutateAsync();
    } catch (e: unknown) {
      message.error(e instanceof Error ? e.message : "check failed");
    }
  };

  const onRun = async () => {
    try {
      const r = await run.mutateAsync();
      setSince(r.started_at);
      message.success("Update started");
    } catch (e: unknown) {
      message.error(e instanceof Error ? e.message : "run failed");
    }
  };

  const onStop = async () => {
    try {
      await stop.mutateAsync();
      message.success("Stop signal sent");
    } catch (e: unknown) {
      message.error(e instanceof Error ? e.message : "stop failed");
    }
  };

  return (
    <Card
      title="Jabali Panel"
      extra={
        <Button
          icon={<ReloadOutlined />}
          onClick={onCheck}
          loading={check.isPending}
          disabled={running}
        >
          Check for updates
        </Button>
      }
    >
      {!result && !running && !finished ? (
        <Typography.Text type="secondary">
          Click "Check for updates" to compare your installation against the
          latest release on origin/main.
        </Typography.Text>
      ) : null}

      {result && result.behind_count === 0 && !running && !finished ? (
        <Alert
          type="success"
          showIcon
          message="Up to date"
          description={
            <span>
              Current commit{" "}
              <Typography.Text code>{result.current_sha.substring(0, 12)}</Typography.Text>{" "}
              matches origin/main.
            </span>
          }
        />
      ) : null}

      {result && result.behind_count > 0 && !running && !finished ? (
        <Alert
          type="warning"
          showIcon
          message={`${result.behind_count} commit${result.behind_count === 1 ? "" : "s"} behind`}
          description={
            <Space direction="vertical">
              <span>
                Local <Typography.Text code>{result.current_sha.substring(0, 12)}</Typography.Text>{" "}
                → remote <Typography.Text code>{result.remote_sha.substring(0, 12)}</Typography.Text>.
              </span>
              <Button
                type="primary"
                icon={<DownloadOutlined />}
                onClick={onRun}
                loading={run.isPending}
              >
                Update Jabali panel
              </Button>
            </Space>
          }
        />
      ) : null}

      {running ? (
        <Alert
          type="info"
          icon={<LoadingOutlined />}
          showIcon
          message="Update in progress"
          description={
            <Space>
              <Button danger size="small" loading={stop.isPending} onClick={onStop}>
                Stop
              </Button>
            </Space>
          }
        />
      ) : null}

      {finished ? (
        <Alert
          type={succeeded ? "success" : "error"}
          showIcon
          message={succeeded ? "Update completed successfully" : "Update failed"}
          description={
            <Space direction="vertical">
              <span>
                Exit code {status.data?.exit_code}. Re-run "Check for updates" to
                refresh status.
              </span>
              <Button
                size="small"
                icon={<ReloadOutlined />}
                onClick={() => {
                  setSince(null);
                  void check.mutateAsync().catch(() => {});
                }}
              >
                Dismiss
              </Button>
            </Space>
          }
        />
      ) : null}

      {since && status.data ? (
        <JobLogTail
          status={status.data.status}
          logTail={status.data.log_tail}
          exitCode={status.data.exit_code}
        />
      ) : null}
    </Card>
  );
}

function AptUpdateCard() {
  const [since, setSince] = useState<string | null>(null);
  const [securityOnly, setSecurityOnly] = useState(false);
  const check = useAptCheck();
  const run = useAptRun();
  const stop = useAptStop();
  const status = useAptStatus(since);

  const result = check.data;
  const running =
    status.data?.status === "active" || status.data?.status === "activating";
  const finished =
    since !== null &&
    status.data !== undefined &&
    !running &&
    status.data.exit_code !== undefined;
  const succeeded = finished && status.data?.exit_code === 0;

  const onCheck = async () => {
    try {
      await check.mutateAsync();
    } catch (e: unknown) {
      message.error(e instanceof Error ? e.message : "check failed");
    }
  };

  const onRun = async () => {
    try {
      const r = await run.mutateAsync();
      setSince(r.started_at);
      message.success("Apt upgrade started");
    } catch (e: unknown) {
      message.error(e instanceof Error ? e.message : "run failed");
    }
  };

  const onStop = async () => {
    try {
      await stop.mutateAsync();
      message.success("Stop signal sent");
    } catch (e: unknown) {
      message.error(e instanceof Error ? e.message : "stop failed");
    }
  };

  return (
    <Card
      title="System Packages"
      extra={
        <Button
          icon={<ReloadOutlined />}
          onClick={onCheck}
          loading={check.isPending}
          disabled={running}
        >
          Check for updates
        </Button>
      }
    >
      {!result && !running && !finished ? (
        <Typography.Text type="secondary">
          Click "Check for updates" to run <code>apt-get update</code> and list
          all upgradable system packages.
        </Typography.Text>
      ) : null}

      {result && result.total === 0 && !running && !finished ? (
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description="System is up to date"
        />
      ) : null}

      {result && result.total > 0 && !running && !finished ? (
        <Space direction="vertical" size={12} style={{ width: "100%" }}>
          <Alert
            type={(result.security_total ?? 0) > 0 ? "error" : "warning"}
            showIcon
            message={`${result.total} package${result.total === 1 ? "" : "s"} can be upgraded${
              (result.security_total ?? 0) > 0 ? ` — ${result.security_total} security` : ""
            }`}
            description="dist-upgrade may pull in libc / openssh / mariadb. Take a snapshot first."
          />
          {(result.security_total ?? 0) > 0 ? (
            <Checkbox
              checked={securityOnly}
              onChange={(e) => setSecurityOnly(e.target.checked)}
            >
              Security updates only
            </Checkbox>
          ) : null}
          <Table<AptPackage>
            rowKey="name"
            size="small"
            dataSource={
              securityOnly ? result.packages.filter((p) => p.security) : result.packages
            }
            pagination={false}
            scroll={{ x: "max-content" }}
            columns={[
              { title: "Package", dataIndex: "name" },
              { title: "Current", dataIndex: "current" },
              { title: "New", dataIndex: "new" },
              { title: "Source", dataIndex: "source", responsive: ["md"] },
              {
                title: "Severity",
                dataIndex: "security",
                width: 110,
                render: (sec: boolean) =>
                  sec ? <Tag color="red">security</Tag> : <Tag>normal</Tag>,
              },
            ]}
          />
          <Button
            type="primary"
            icon={<DownloadOutlined />}
            onClick={onRun}
            loading={run.isPending}
          >
            Apply updates
          </Button>
        </Space>
      ) : null}

      {running ? (
        <Alert
          type="info"
          icon={<LoadingOutlined />}
          showIcon
          message="Apt upgrade in progress"
          description={
            <Button danger size="small" loading={stop.isPending} onClick={onStop}>
              Stop
            </Button>
          }
        />
      ) : null}

      {finished ? (
        <Alert
          type={succeeded ? "success" : "error"}
          showIcon
          message={succeeded ? "Apt upgrade completed successfully" : "Apt upgrade failed"}
          description={
            <Space direction="vertical">
              <span>
                Exit code {status.data?.exit_code}. Re-run "Check for updates" to
                refresh upgradable list.
              </span>
              <Button
                size="small"
                icon={<ReloadOutlined />}
                onClick={() => {
                  setSince(null);
                  void check.mutateAsync().catch(() => {});
                }}
              >
                Dismiss
              </Button>
            </Space>
          }
        />
      ) : null}

      {since && status.data ? (
        <JobLogTail
          status={status.data.status}
          logTail={status.data.log_tail}
          exitCode={status.data.exit_code}
        />
      ) : null}
    </Card>
  );
}

// AutomaticUpdatesCard — toggle + schedule unattended apt security upgrades
// and (opt-in, default off) jabali self-update. Saves the desired state; the
// autoupdate reconciler converges it onto the host.
function AutomaticUpdatesCard() {
  const { data, isLoading } = useAutoupdateConfig();
  const save = useUpdateAutoupdate();
  const [draft, setDraft] = useState<AutoupdateConfig | null>(null);
  const cfg = draft ?? data ?? null;

  const dirty =
    draft !== null && data !== undefined && JSON.stringify(draft) !== JSON.stringify(data);

  const patch = (p: Partial<AutoupdateConfig>) => {
    if (!cfg) return;
    setDraft({ ...cfg, ...p });
  };

  const onSave = async () => {
    if (!cfg) return;
    try {
      await save.mutateAsync(cfg);
      setDraft(null);
      message.success("Automatic updates saved");
    } catch (e: unknown) {
      message.error(e instanceof Error ? e.message : "save failed");
    }
  };

  return (
    <Card title={<span><SettingOutlined /> Automatic Updates</span>}>
      {isLoading || !cfg ? (
        <Typography.Text type="secondary">Loading…</Typography.Text>
      ) : (
        <Space direction="vertical" size={16} style={{ width: "100%" }}>
          <Row align="middle" gutter={[12, 12]}>
            <Col flex="auto">
              <Space direction="vertical" size={0}>
                <Typography.Text strong>OS security updates</Typography.Text>
                <Typography.Text type="secondary">
                  Apply Debian security patches automatically via
                  unattended-upgrades.
                </Typography.Text>
              </Space>
            </Col>
            <Col>
              <Switch
                checked={cfg.apt_enabled}
                onChange={(v) => patch({ apt_enabled: v })}
              />
            </Col>
            <Col>
              <TimePicker
                format="HH:mm"
                minuteStep={15}
                allowClear={false}
                disabled={!cfg.apt_enabled}
                value={dayjs(cfg.apt_time, "HH:mm")}
                onChange={(d) => patch({ apt_time: d ? d.format("HH:mm") : cfg.apt_time })}
              />
            </Col>
          </Row>

          <Row align="middle" gutter={[12, 12]}>
            <Col flex="auto">
              <Space direction="vertical" size={0}>
                <Typography.Text strong>
                  Jabali panel self-update{" "}
                  <Tooltip title="A bad self-update can take the panel offline. Leave off unless you actively want hands-off panel upgrades.">
                    <Tag color="orange">advanced</Tag>
                  </Tooltip>
                </Typography.Text>
                <Typography.Text type="secondary">
                  Run <code>jabali update</code> automatically on a schedule.
                </Typography.Text>
              </Space>
            </Col>
            <Col>
              <Switch
                checked={cfg.jabali_enabled}
                onChange={(v) => patch({ jabali_enabled: v })}
              />
            </Col>
            <Col>
              <TimePicker
                format="HH:mm"
                minuteStep={15}
                allowClear={false}
                disabled={!cfg.jabali_enabled}
                value={dayjs(cfg.jabali_time, "HH:mm")}
                onChange={(d) =>
                  patch({ jabali_time: d ? d.format("HH:mm") : cfg.jabali_time })
                }
              />
            </Col>
          </Row>

          <Button
            type="primary"
            onClick={onSave}
            disabled={!dirty}
            loading={save.isPending}
          >
            Save
          </Button>
        </Space>
      )}
    </Card>
  );
}

const historyStatusTag = (status: string) => {
  if (status === "success") return <Tag color="green">success</Tag>;
  if (status === "failed") return <Tag color="red">failed</Tag>;
  return <Tag color="processing">running</Tag>;
};

// RecentHistoryCard — the persisted update_history log (runs only). Survives
// page reloads because panel-api logs the row and the run reconciler marks it
// finished, regardless of whether this page was open.
function RecentHistoryCard() {
  const { data, isLoading } = useUpdateHistory(20);
  const rows = data?.items ?? [];
  return (
    <Card title={<span><ClockCircleOutlined /> Recent History</span>}>
      {isLoading ? (
        <Typography.Text type="secondary">Loading…</Typography.Text>
      ) : rows.length === 0 ? (
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="No update runs yet" />
      ) : (
        <Table<UpdateHistoryRow>
          rowKey="id"
          size="small"
          dataSource={rows}
          pagination={false}
          scroll={{ x: "max-content" }}
          columns={[
            {
              title: "When",
              dataIndex: "started_at",
              render: (t: string) => dayjs(t).format("YYYY-MM-DD HH:mm"),
            },
            {
              title: "Target",
              dataIndex: "kind",
              render: (k: string) => (k === "jabali" ? "Jabali panel" : "OS packages"),
            },
            { title: "Action", dataIndex: "action" },
            { title: "Status", dataIndex: "status", render: historyStatusTag },
            { title: "Summary", dataIndex: "summary" },
          ]}
        />
      )}
    </Card>
  );
}

// ChangelogCard — release notes from the public GitHub mirror. Empty until the
// first release is cut.
function ChangelogCard() {
  const { data, isLoading } = useChangelog();
  const items = data?.items ?? [];
  return (
    <Card title={<span><FileTextOutlined /> Changelog</span>}>
      {isLoading ? (
        <Typography.Text type="secondary">Loading…</Typography.Text>
      ) : items.length === 0 ? (
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="No releases yet" />
      ) : (
        <Timeline
          items={items.map((e) => ({
            children: (
              <Space direction="vertical" size={2} style={{ width: "100%" }}>
                <Typography.Text strong>
                  {e.name || e.tag}{" "}
                  <Typography.Text type="secondary">
                    {e.published_at ? dayjs(e.published_at).format("YYYY-MM-DD") : ""}
                  </Typography.Text>
                </Typography.Text>
                {e.body ? (
                  <Typography.Paragraph
                    type="secondary"
                    ellipsis={{ rows: 4, expandable: true, symbol: "more" }}
                    style={{ marginBottom: 0, whiteSpace: "pre-wrap" }}
                  >
                    {e.body}
                  </Typography.Paragraph>
                ) : null}
              </Space>
            ),
          }))}
        />
      )}
    </Card>
  );
}
