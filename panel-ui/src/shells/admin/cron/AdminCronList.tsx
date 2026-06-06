// AdminCronList — admin-side cross-tenant cron view (GH#127).
//
// Mirrors UserCronList's read + lifecycle actions but adds an Owner
// column and skips the "New Cron Job" button — admin-create-as-user
// is out of scope for the initial cut. PATCH / DELETE / run-now / log
// all go through the existing /cron/:id endpoints, which authorise
// admins via fetchAndAuthorize's claims.IsAdmin bypass.
import { useMemo, useState } from "react";
import { App, Button, Card, Input, Popconfirm, Space, Switch, Table, Tag, Tooltip, Typography } from "antd";
import {
  CalendarCheckOutlined,
  PlusSquareOutlined,
  DeleteOutlined,
  PlayCircleOutlined,
  EyeOutlined,
  CheckOutlined,
  CloseOutlined,
} from "@icons";
import { RowActionButton } from "../../../components/RowActionButton";
import { useQuery } from "@tanstack/react-query";
import dayjs from "dayjs";
import relativeTime from "dayjs/plugin/relativeTime";
import {
  listAdminCronJobs,
  deleteCronJob,
  updateCronJob,
  runCronJobNow,
  type AdminCronJob,
} from "../../../apiClient";
import { CronLogDrawer } from "../../user/cron/CronLogDrawer";
import { RunNowResultModal } from "../../user/cron/RunNowResultModal";
import { AdminCreateCronModal } from "./AdminCreateCronModal";
import { EmptyWithCTA } from "../../../components/EmptyWithCTA";

dayjs.extend(relativeTime);

const SCHEDULE_PRESETS: Record<string, string> = {
  "0 * * * *": "Hourly",
  "0 3 * * *": "Daily at 3 AM",
  "0 3 * * 0": "Weekly (Sun 3 AM)",
  "0 3 1 * *": "Monthly (1st 3 AM)",
};
const humanizeSchedule = (s: string): string => SCHEDULE_PRESETS[s] || s;

export const AdminCronList = () => {
  const { message: antMessage } = App.useApp();
  const [logDrawerOpen, setLogDrawerOpen] = useState(false);
  const [logJobId, setLogJobId] = useState<string | null>(null);
  const [runNowModalOpen, setRunNowModalOpen] = useState(false);
  const [runNowResult, setRunNowResult] = useState<{ exit_code: number; stdout: string; stderr: string } | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [runningId, setRunningId] = useState<string | null>(null);
  const [togglingId, setTogglingId] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [createOpen, setCreateOpen] = useState(false);

  const { data: listResponse = { items: [] }, isLoading, refetch } = useQuery({
    queryKey: ["admin-cron-jobs"],
    queryFn: async () => listAdminCronJobs(),
  });

  const jobs = listResponse.items || [];
  const filteredJobs = useMemo(() => {
    if (!search) return jobs;
    const needle = search.toLowerCase();
    return jobs.filter(
      (j) =>
        j.name.toLowerCase().includes(needle) ||
        j.command.toLowerCase().includes(needle) ||
        j.schedule.toLowerCase().includes(needle) ||
        (j.username || "").toLowerCase().includes(needle),
    );
  }, [jobs, search]);

  const surfaceErr = (err: unknown, fallback: string) => {
    const msg =
      (err as { response?: { data?: { detail?: string } }; message?: string })?.response?.data?.detail ??
      (err as { message?: string })?.message ??
      fallback;
    antMessage.error(msg);
  };

  const handleDelete = async (job: AdminCronJob) => {
    setDeletingId(job.id);
    try {
      await deleteCronJob(job.id);
      antMessage.success("Cron job deleted");
      refetch();
    } catch (e) {
      surfaceErr(e, "Failed to delete cron job");
    } finally {
      setDeletingId(null);
    }
  };

  const handleRunNow = async (job: AdminCronJob) => {
    setRunningId(job.id);
    try {
      const result = await runCronJobNow(job.id);
      setRunNowResult(result);
      setRunNowModalOpen(true);
      setTimeout(() => refetch(), 2000);
    } catch (e) {
      surfaceErr(e, "Failed to run cron job");
    } finally {
      setRunningId(null);
    }
  };

  const handleToggleEnabled = async (job: AdminCronJob) => {
    setTogglingId(job.id);
    try {
      await updateCronJob(job.id, { enabled: !job.enabled });
      antMessage.success(job.enabled ? "Cron job disabled" : "Cron job enabled");
      refetch();
    } catch (e) {
      surfaceErr(e, "Failed to update cron job");
    } finally {
      setTogglingId(null);
    }
  };

  const truncate = (s: string, n = 40) => (s.length <= n ? s : s.substring(0, n) + "…");

  return (
    <div>
      <Space wrap align="center" style={{ marginBottom: 16, width: "100%", justifyContent: "space-between" }}>
        <Typography.Title level={3} style={{ margin: 0 }}>
          <CalendarCheckOutlined /> Cron Jobs (all tenants)
        </Typography.Title>
        <Space>
          <Typography.Text type="secondary">
            {jobs.length} job{jobs.length === 1 ? "" : "s"}
          </Typography.Text>
          <Button
            type="primary"
            icon={<PlusSquareOutlined />}
            onClick={() => setCreateOpen(true)}
          >
            New Cron Job
          </Button>
        </Space>
      </Space>

      <Card>
        <Space direction="vertical" size="middle" style={{ width: "100%" }}>
          <Input.Search
            placeholder="Search by name, command, schedule, or owner"
            allowClear
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            style={{ maxWidth: 480 }}
          />
          <Table<AdminCronJob>
            rowKey="id"
            size="small"
            loading={isLoading}
            dataSource={filteredJobs}
            pagination={{ pageSize: 25, showSizeChanger: true }}
            scroll={{ x: "max-content" }}
            locale={{
              emptyText: (
                <EmptyWithCTA
                  description="No cron jobs yet"
                  ctaLabel="Create cron job"
                  onCta={() => setCreateOpen(true)}
                />
              ),
            }}
            columns={[
              {
                title: "Owner",
                dataIndex: "username",
                width: 160,
                render: (u: string, row) => (
                  <Tooltip title={`user_id: ${row.user_id}`}>
                    <Typography.Text code>{u || row.user_id}</Typography.Text>
                  </Tooltip>
                ),
              },
              {
                title: "Name",
                dataIndex: "name",
                render: (name: string) => <Typography.Text strong>{name}</Typography.Text>,
              },
              {
                title: "Schedule",
                dataIndex: "schedule",
                width: 200,
                render: (sch: string) => (
                  <Tooltip title={sch}>
                    <Tag>{humanizeSchedule(sch)}</Tag>
                  </Tooltip>
                ),
              },
              {
                title: "Command",
                dataIndex: "command",
                render: (cmd: string) => (
                  <Tooltip title={cmd}>
                    <Typography.Text code>{truncate(cmd)}</Typography.Text>
                  </Tooltip>
                ),
              },
              {
                title: "Enabled",
                dataIndex: "enabled",
                width: 90,
                render: (_: boolean, row) => (
                  <Switch
                    size="small"
                    checked={row.enabled}
                    loading={togglingId === row.id}
                    onChange={() => handleToggleEnabled(row)}
                    checkedChildren={<CheckOutlined />}
                    unCheckedChildren={<CloseOutlined />}
                  />
                ),
              },
              {
                title: "Last run",
                dataIndex: "last_run_at",
                width: 160,
                render: (ts: string | null, row) =>
                  ts ? (
                    <Tooltip title={`exit=${row.last_exit_code ?? "?"}${row.last_error ? "\n" + row.last_error : ""}`}>
                      <Tag color={row.last_exit_code === 0 ? "green" : row.last_exit_code === null ? "default" : "red"}>
                        {dayjs(ts).fromNow()}
                      </Tag>
                    </Tooltip>
                  ) : (
                    <Tag>never</Tag>
                  ),
              },
              {
                title: "Actions",
                width: 240,
                render: (_, row) => (
                  <Space size={4}>
                    <RowActionButton
                      icon={<PlayCircleOutlined />}
                      loading={runningId === row.id}
                      onClick={() => handleRunNow(row)}
                    >
                      Run
                    </RowActionButton>
                    <RowActionButton
                      icon={<EyeOutlined />}
                      onClick={() => {
                        setLogJobId(row.id);
                        setLogDrawerOpen(true);
                      }}
                    >
                      Log
                    </RowActionButton>
                    <Popconfirm
                      title={`Delete cron job "${row.name}"?`}
                      okText="Delete"
                      okButtonProps={{ danger: true }}
                      onConfirm={() => handleDelete(row)}
                    >
                      <RowActionButton
                        icon={<DeleteOutlined />}
                        loading={deletingId === row.id}
                        danger
                      />
                    </Popconfirm>
                  </Space>
                ),
              },
            ]}
          />
        </Space>
      </Card>

      <AdminCreateCronModal
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        onSuccess={() => {
          setCreateOpen(false);
          refetch();
        }}
      />

      {logJobId && (
        <CronLogDrawer
          open={logDrawerOpen}
          jobId={logJobId}
          onClose={() => {
            setLogDrawerOpen(false);
            setLogJobId(null);
          }}
        />
      )}
      <RunNowResultModal
        open={runNowModalOpen}
        result={runNowResult}
        onClose={() => {
          setRunNowModalOpen(false);
          setRunNowResult(null);
        }}
      />


    </div>
  );
};
