// SystemUpdatesPage — admin Updates Center (M53, ADR-0118).
//
// Layout mirrors the product mockup: a row of 4 stat cards, then a 2-up grid
// of Jabali Panel + System Packages, then Automatic Updates + Recent History,
// then the Changelog. Data auto-loads on mount (jabali + apt checks fire once)
// so the page shows real numbers immediately instead of empty placeholders.
import { useEffect, useState, type CSSProperties } from "react";
import {
  Alert,
  Button,
  Card,
  Checkbox,
  Col,
  Empty,
  Row,
  Space,
  Spin,
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
  AppstoreOutlined,
  ClockCircleOutlined,
  CodeOutlined,
  DownloadOutlined,
  LoadingOutlined,
  ReloadOutlined,
  SafetyOutlined,
  SettingOutlined,
} from "@icons";

import { JobLogTail } from "../../../components/JobLogTail";
import { RepairCard } from "./RepairCard";
import {
  useAptCheck,
  useAptRun,
  useAptStatus,
  useAptStop,
  useAutoupdateConfig,
  useJabaliCheck,
  useJabaliRun,
  useJabaliStatus,
  useJabaliStop,
  useUpdateAutoupdate,
  useUpdateHistory,
  useUpdateState,
  type AptCheckResult,
  type AptPackage,
  type AutoupdateConfig,
  type JabaliCheckResult,
  type UpdateHistoryRow,
} from "../../../hooks/useSystemUpdates";

const { Text, Title } = Typography;

function timeAgo(iso?: string): string {
  if (!iso) return "Never";
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return "Never";
  const s = Math.floor((Date.now() - then) / 1000);
  if (s < 60) return "Just now";
  if (s < 3600) return `${Math.floor(s / 60)} min ago`;
  if (s < 86400) return `${Math.floor(s / 3600)} h ago`;
  return `${Math.floor(s / 86400)} d ago`;
}

const sha8 = (s?: string) => (s ? s.substring(0, 8) : "—");

export const SystemUpdatesPage = () => {
  const state = useUpdateState();
  const jabali = useJabaliCheck();
  const apt = useAptCheck();

  // Auto-load once on mount so cards/tables show data without a manual click.
  useEffect(() => {
    jabali.mutate();
    apt.mutate();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const onCheckNow = async () => {
    try {
      await Promise.all([jabali.mutateAsync(), apt.mutateAsync()]);
      await state.refetch();
      message.success("Checked for updates");
    } catch (e: unknown) {
      message.error(e instanceof Error ? e.message : "check failed");
    }
  };

  return (
    <div>
      <Title level={3} style={{ marginTop: 0, marginBottom: 16 }}>
        <DownloadOutlined /> Updates
      </Title>
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <StatCards
          state={state.data}
          jabali={jabali.data}
          apt={apt.data}
          checking={jabali.isPending || apt.isPending}
          onCheckNow={onCheckNow}
        />
        <Row gutter={[16, 16]}>
          <Col xs={24} xl={12}>
            <JabaliPanelCard check={jabali} />
          </Col>
          <Col xs={24} xl={12}>
            <SystemPackagesCard check={apt} />
          </Col>
        </Row>
        <RepairCard />
        <Row gutter={[16, 16]}>
          <Col xs={24} xl={12}>
            <AutomaticUpdatesCard />
          </Col>
          <Col xs={24} xl={12}>
            <RecentHistoryCard />
          </Col>
        </Row>
      </Space>
    </div>
  );
};

// --- stat cards ------------------------------------------------------------

type StatCardsProps = {
  state?: {
    jabali_behind: number;
    jabali_current_sha?: string;
    apt_total: number;
    apt_security: number;
    apt_checked_at?: string;
    jabali_checked_at?: string;
  };
  jabali?: JabaliCheckResult;
  apt?: AptCheckResult;
  checking: boolean;
  onCheckNow: () => void;
};

function StatCards({ state, jabali, apt, checking, onCheckNow }: StatCardsProps) {
  const behind = jabali?.behind_count ?? state?.jabali_behind ?? 0;
  const aptTotal = apt?.total ?? state?.apt_total ?? 0;
  const aptSecurity = apt?.security_total ?? state?.apt_security ?? 0;
  const installed = apt?.installed_total;
  const lastChecked = state?.apt_checked_at ?? state?.jabali_checked_at;

  const cardBody: CSSProperties = { display: "flex", gap: 14, alignItems: "flex-start" };
  // Background is a translucent tint of the icon color so the box reads the
  // same on light and dark themes (hardcoded pastels were invisible/clashing
  // in dark mode). `${color}22` ~= 13% alpha.
  const iconBox = (color: string): CSSProperties => ({
    width: 44,
    height: 44,
    borderRadius: 10,
    background: `${color}22`,
    color,
    display: "flex",
    alignItems: "center",
    justifyContent: "center",
    fontSize: 20,
    flex: "none",
  });

  return (
    <Row gutter={[16, 16]}>
      <Col xs={24} sm={12} xl={6}>
        <Card style={{ height: "100%" }}>
          <div style={cardBody}>
            <span style={iconBox("#2563eb")}>
              <CodeOutlined />
            </span>
            <div>
              <Text type="secondary">Panel Version</Text>
              <div style={{ margin: "4px 0" }}>
                <Text strong style={{ fontSize: 20 }}>{sha8(jabali?.current_sha ?? state?.jabali_current_sha)}</Text>{" "}
                {behind === 0 ? (
                  <Tag color="green">Up to date</Tag>
                ) : (
                  <Tag color="orange">{behind} behind</Tag>
                )}
              </div>
              <Text type="secondary" style={{ fontSize: 12 }}>
                Latest: {sha8(jabali?.remote_sha)}
              </Text>
            </div>
          </div>
        </Card>
      </Col>

      <Col xs={24} sm={12} xl={6}>
        <Card style={{ height: "100%" }}>
          <div style={cardBody}>
            <span style={iconBox("#7c3aed")}>
              <AppstoreOutlined />
            </span>
            <div>
              <Text type="secondary">System Packages</Text>
              <div style={{ margin: "4px 0" }}>
                <Text strong style={{ fontSize: 20 }}>{aptTotal}</Text>{" "}
                {aptTotal > 0 ? (
                  <Tag color="orange">Updates available</Tag>
                ) : (
                  <Tag color="green">Up to date</Tag>
                )}
              </div>
              <Text type="secondary" style={{ fontSize: 12 }}>
                {installed ? `of ${installed} packages` : "installed packages"}
              </Text>
            </div>
          </div>
        </Card>
      </Col>

      <Col xs={24} sm={12} xl={6}>
        <Card style={{ height: "100%" }}>
          <div style={cardBody}>
            <span style={iconBox("#dc2626")}>
              <SafetyOutlined />
            </span>
            <div>
              <Text type="secondary">Security Updates</Text>
              <div style={{ margin: "4px 0" }}>
                <Text strong style={{ fontSize: 20, color: aptSecurity > 0 ? "#cf1322" : undefined }}>
                  {aptSecurity}
                </Text>{" "}
                {aptSecurity > 0 ? <Tag color="red">Critical</Tag> : <Tag color="green">Clear</Tag>}
              </div>
              <Text type="secondary" style={{ fontSize: 12 }}>
                {aptSecurity > 0 ? "Important security fixes" : "No security updates"}
              </Text>
            </div>
          </div>
        </Card>
      </Col>

      <Col xs={24} sm={12} xl={6}>
        <Card style={{ height: "100%" }}>
          <div style={cardBody}>
            <span style={iconBox("#16a34a")}>
              <ClockCircleOutlined />
            </span>
            <div style={{ flex: "auto", minWidth: 0 }}>
              <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 8 }}>
                <Text type="secondary" style={{ whiteSpace: "nowrap" }}>Last Checked</Text>
                <Tooltip title="Check for updates">
                  <Button
                    type="text"
                    size="small"
                    icon={<ReloadOutlined />}
                    loading={checking}
                    onClick={onCheckNow}
                    aria-label="Check for updates"
                  />
                </Tooltip>
              </div>
              <div style={{ margin: "4px 0" }}>
                <Text strong style={{ fontSize: 18 }}>{timeAgo(lastChecked)}</Text>
              </div>
              <Text type="secondary" style={{ fontSize: 12 }}>
                {lastChecked ? dayjs(lastChecked).format("MMM D, YYYY HH:mm") : "—"}
              </Text>
            </div>
          </div>
        </Card>
      </Col>
    </Row>
  );
}

// --- Jabali Panel card -----------------------------------------------------

function JabaliPanelCard({ check }: { check: ReturnType<typeof useJabaliCheck> }) {
  const [since, setSince] = useState<string | null>(null);
  const run = useJabaliRun();
  const stop = useJabaliStop();
  const status = useJabaliStatus(since);

  const result = check.data;
  const behind = result?.behind_count ?? 0;
  const running = status.data?.status === "active" || status.data?.status === "activating";
  const finished =
    since !== null && status.data !== undefined && !running && status.data.exit_code !== undefined;
  const succeeded = finished && status.data?.exit_code === 0;
  const commits = result?.recent_commits ?? [];

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
      style={{ height: "100%" }}
      title={
        <Space>
          <CodeOutlined />
          <span>Jabali Panel</span>
          {check.isPending ? (
            <Tag>checking…</Tag>
          ) : behind === 0 ? (
            <Tag color="green">Up to date</Tag>
          ) : (
            <Tag color="orange">{behind} behind</Tag>
          )}
        </Space>
      }
    >
      <Row gutter={16}>
        <Col span={12}>
          <Text type="secondary">Current Version</Text>
          <div>
            <Text strong code style={{ fontSize: 16 }}>{sha8(result?.current_sha)}</Text>
          </div>
          {result?.branch ? (
            <Text type="secondary" style={{ fontSize: 12 }}>branch {result.branch}</Text>
          ) : null}
        </Col>
        <Col span={12}>
          <Text type="secondary">Latest Version</Text>
          <div>
            <Text strong code style={{ fontSize: 16 }}>{sha8(result?.remote_sha)}</Text>
          </div>
          <Text type="secondary" style={{ fontSize: 12 }}>
            {behind === 0 ? "No updates available" : `${behind} commit${behind === 1 ? "" : "s"} behind`}
          </Text>
        </Col>
      </Row>

      {running ? (
        <Alert
          style={{ marginTop: 16 }}
          type="info"
          icon={<LoadingOutlined />}
          showIcon
          message="Update in progress"
          description={
            <Button danger size="small" loading={stop.isPending} onClick={onStop}>
              Stop
            </Button>
          }
        />
      ) : finished ? (
        <Alert
          style={{ marginTop: 16 }}
          type={succeeded ? "success" : "error"}
          showIcon
          message={succeeded ? "Update completed" : "Update failed"}
          description={`Exit code ${status.data?.exit_code}.`}
        />
      ) : null}

      {status.data && (running || since) ? (
        <JobLogTail
          status={status.data.status}
          logTail={status.data.log_tail}
          exitCode={status.data.exit_code}
        />
      ) : null}

      <Space style={{ marginTop: 16 }}>
        <Button icon={<ReloadOutlined />} loading={check.isPending} onClick={() => check.mutate()}>
          Check for updates
        </Button>
        <Button
          type="primary"
          icon={<DownloadOutlined />}
          loading={run.isPending}
          disabled={running || behind === 0}
          onClick={onRun}
        >
          Update now
        </Button>
      </Space>

      {commits.length > 0 ? (
        <div style={{ marginTop: 16 }}>
          <Text type="secondary" style={{ fontSize: 12 }}>Recent changes</Text>
          <ul style={{ margin: "6px 0 0", paddingLeft: 18 }}>
            {commits.map((c) => (
              <li key={c.sha} style={{ marginBottom: 4 }}>
                <Text>{c.subject}</Text>{" "}
                <Text type="secondary" code style={{ fontSize: 11 }}>{c.sha}</Text>{" "}
                <Text type="secondary" style={{ fontSize: 11 }}>
                  {c.date ? dayjs(c.date).format("MMM D") : ""}
                </Text>
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </Card>
  );
}

// --- System Packages card --------------------------------------------------

function SystemPackagesCard({ check }: { check: ReturnType<typeof useAptCheck> }) {
  const [since, setSince] = useState<string | null>(null);
  const [securityOnly, setSecurityOnly] = useState(false);
  const run = useAptRun();
  const stop = useAptStop();
  const status = useAptStatus(since);

  const result = check.data;
  const running = status.data?.status === "active" || status.data?.status === "activating";
  const finished =
    since !== null && status.data !== undefined && !running && status.data.exit_code !== undefined;

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

  const rows = result
    ? securityOnly
      ? result.packages.filter((p) => p.security)
      : result.packages
    : [];

  return (
    <Card
      style={{ height: "100%" }}
      title={
        <Space>
          <AppstoreOutlined />
          <span>System Packages</span>
        </Space>
      }
      extra={
        result ? (
          <Space size={4}>
            {result.total > 0 ? <Tag color="orange">{result.total} updates available</Tag> : null}
            {(result.security_total ?? 0) > 0 ? (
              <Tag color="red">{result.security_total} critical</Tag>
            ) : null}
          </Space>
        ) : null
      }
    >
      {result?.installed_total ? (
        <Text type="secondary">{result.installed_total} packages installed</Text>
      ) : null}

      {check.isPending && !result ? (
        <div style={{ textAlign: "center", padding: 24 }}>
          <Spin />
        </div>
      ) : result && result.total === 0 ? (
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="System is up to date" />
      ) : result ? (
        <Space direction="vertical" size={12} style={{ width: "100%", marginTop: 8 }}>
          {(result.security_total ?? 0) > 0 ? (
            <Checkbox checked={securityOnly} onChange={(e) => setSecurityOnly(e.target.checked)}>
              Security updates only
            </Checkbox>
          ) : null}
          <Table<AptPackage>
            rowKey="name"
            size="small"
            dataSource={rows}
            pagination={rows.length > 8 ? { pageSize: 8, size: "small" } : false}
            scroll={{ x: "max-content" }}
            columns={[
              { title: "Package", dataIndex: "name" },
              { title: "Current Version", dataIndex: "current" },
              { title: "Latest Version", dataIndex: "new" },
              {
                title: "Type",
                dataIndex: "security",
                width: 100,
                render: (sec: boolean) =>
                  sec ? <Tag color="red">Security</Tag> : <Tag color="orange">Update</Tag>,
              },
            ]}
          />
        </Space>
      ) : null}

      {running ? (
        <Alert
          style={{ marginTop: 12 }}
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
      {status.data && (running || since) ? (
        <JobLogTail
          status={status.data.status}
          logTail={status.data.log_tail}
          exitCode={status.data.exit_code}
        />
      ) : null}

      <Space style={{ marginTop: 12 }}>
        <Button icon={<ReloadOutlined />} loading={check.isPending} onClick={() => check.mutate()}>
          Check for updates
        </Button>
        {result && result.total > 0 ? (
          <Button
            type="primary"
            icon={<DownloadOutlined />}
            loading={run.isPending}
            disabled={running || finished}
            onClick={onRun}
          >
            Apply updates
          </Button>
        ) : null}
      </Space>
    </Card>
  );
}

// --- Automatic Updates -----------------------------------------------------

function AutomaticUpdatesCard() {
  const { data, isLoading } = useAutoupdateConfig();
  const save = useUpdateAutoupdate();
  const [draft, setDraft] = useState<AutoupdateConfig | null>(null);
  const cfg = draft ?? data ?? null;
  const dirty = draft !== null && data !== undefined && JSON.stringify(draft) !== JSON.stringify(data);

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
    <Card
      style={{ height: "100%" }}
      title={
        <Space>
          <SettingOutlined />
          <span>Automatic Updates</span>
        </Space>
      }
    >
      {isLoading || !cfg ? (
        <div style={{ textAlign: "center", padding: 24 }}><Spin /></div>
      ) : (
        <Space direction="vertical" size={16} style={{ width: "100%" }}>
          <Row align="middle" gutter={[12, 12]}>
            <Col flex="auto">
              <Text strong>OS security updates</Text>
              <div>
                <Text type="secondary" style={{ fontSize: 12 }}>
                  Apply Debian security patches automatically via unattended-upgrades.
                </Text>
              </div>
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
            <Col>
              <Switch checked={cfg.apt_enabled} onChange={(v) => patch({ apt_enabled: v })} />
            </Col>
          </Row>

          <Row align="middle" gutter={[12, 12]}>
            <Col flex="auto">
              <Text strong>
                Jabali panel self-update{" "}
                <Tooltip title="A bad self-update can take the panel offline. Leave off unless you want hands-off panel upgrades.">
                  <Tag color="orange">advanced</Tag>
                </Tooltip>
              </Text>
              <div>
                <Text type="secondary" style={{ fontSize: 12 }}>
                  Run <code>jabali update</code> automatically on a schedule.
                </Text>
              </div>
            </Col>
            <Col>
              <TimePicker
                format="HH:mm"
                minuteStep={15}
                allowClear={false}
                disabled={!cfg.jabali_enabled}
                value={dayjs(cfg.jabali_time, "HH:mm")}
                onChange={(d) => patch({ jabali_time: d ? d.format("HH:mm") : cfg.jabali_time })}
              />
            </Col>
            <Col>
              <Switch checked={cfg.jabali_enabled} onChange={(v) => patch({ jabali_enabled: v })} />
            </Col>
          </Row>

          <Button type="primary" onClick={onSave} disabled={!dirty} loading={save.isPending}>
            Save
          </Button>
        </Space>
      )}
    </Card>
  );
}

// --- Recent History --------------------------------------------------------

const historyStatusTag = (status: string) => {
  if (status === "success") return <Tag color="green">success</Tag>;
  if (status === "failed") return <Tag color="red">failed</Tag>;
  return <Tag color="processing">running</Tag>;
};

function RecentHistoryCard() {
  const { data, isLoading } = useUpdateHistory(8);
  const rows = data?.items ?? [];
  return (
    <Card
      style={{ height: "100%" }}
      title={
        <Space>
          <ClockCircleOutlined />
          <span>Recent Update History</span>
        </Space>
      }
    >
      {isLoading ? (
        <div style={{ textAlign: "center", padding: 24 }}><Spin /></div>
      ) : rows.length === 0 ? (
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="No update runs yet" />
      ) : (
        <Timeline
          items={rows.map((r: UpdateHistoryRow) => ({
            color: r.status === "success" ? "green" : r.status === "failed" ? "red" : "blue",
            children: (
              <Space direction="vertical" size={0}>
                <Space>
                  <Text>{dayjs(r.started_at).format("MMM D, YYYY HH:mm")}</Text>
                  {historyStatusTag(r.status)}
                </Space>
                <Text type="secondary">
                  {r.kind === "jabali" ? "Jabali panel" : "System packages"} {r.action}
                  {r.summary ? ` — ${r.summary}` : ""}
                </Text>
              </Space>
            ),
          }))}
        />
      )}
    </Card>
  );
}
