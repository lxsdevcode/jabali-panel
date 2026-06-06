// LogsDrawer — tail docker compose logs for one installed app.
// 5s poll while the drawer is open. ~10k-line ceiling matches the
// agent cap.
import { Button, Drawer, InputNumber, Space, Typography } from "antd";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { fetchLogs } from "./api";

interface Props {
  open: boolean;
  appId: string | null;
  appName?: string;
  lastError?: string | null;
  onClose: () => void;
}

export const LogsDrawer = ({ open, appId, appName, lastError, onClose }: Props) => {
  const [lines, setLines] = useState(500);

  const { data, isFetching, refetch } = useQuery({
    queryKey: ["docker-app-logs", appId, lines],
    queryFn: () => fetchLogs(appId!, lines),
    enabled: !!appId && open,
    refetchInterval: open ? 5000 : false,
  });

  return (
    <Drawer
      open={open}
      onClose={onClose}
      title={appName ? `Container logs · ${appName}` : "Container logs"}
      width="min(100%, 840px)"
      destroyOnClose
      extra={
        <Space>
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            Lines
          </Typography.Text>
          <InputNumber
            size="small"
            min={50}
            max={10000}
            step={500}
            value={lines}
            onChange={(v) => setLines(v ?? 500)}
            style={{ width: 90 }}
          />
          <Button size="small" loading={isFetching} onClick={() => refetch()}>
            Refresh
          </Button>
        </Space>
      }
    >
      {!data?.logs && lastError && (
        <div style={{ background: "rgba(207,19,34,0.08)", border: "1px solid #cf1322", padding: 12, borderRadius: 6, marginBottom: 12, fontSize: 12 }}>
          <div style={{ fontWeight: 600, marginBottom: 4 }}>Container never produced logs.</div>
          <div style={{ marginBottom: 4 }}>Recorded install / startup error:</div>
          <code style={{ whiteSpace: "pre-wrap", wordBreak: "break-word" }}>{lastError}</code>
        </div>
      )}
      <pre
        style={{
          background: "var(--ant-color-bg-elevated, #0d1117)",
          color: "var(--ant-color-text-base, #c9d1d9)",
          padding: 12,
          borderRadius: 6,
          fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace",
          fontSize: 12,
          lineHeight: 1.5,
          margin: 0,
          maxHeight: "calc(100vh - 200px)",
          overflow: "auto",
          whiteSpace: "pre-wrap",
        }}
      >
        {data?.logs ?? (isFetching ? "Loading…" : "No logs.")}
      </pre>
    </Drawer>
  );
};
