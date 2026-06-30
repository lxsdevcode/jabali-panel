// DomainCacheSection — per-domain nginx FastCGI micro-cache toggle + TTL
// (ADR-0108, Gitea #596). Self-contained like DomainEmailSection: loads its own
// state, flips the switch, edits the cache TTL (DB-as-truth → reconciler
// re-renders the vhost), and exposes a manual purge button.
import { useCallback, useEffect, useState } from "react";
import {
  Alert,
  Button,
  InputNumber,
  Popconfirm,
  Skeleton,
  Space,
  Switch,
  message,
} from "antd";
import { CheckOutlined, CloseOutlined, ThunderboltOutlined } from "@icons";

import { apiClient } from "../apiClient";

type CacheState = {
  domain_id: string;
  domain_name: string;
  enabled: boolean;
  ttl_seconds: number;
};

type Props = { domainId: string };

const TTL_MIN = 10;
const TTL_MAX = 86400;

export const DomainCacheSection = ({ domainId }: Props) => {
  const [state, setState] = useState<CacheState | null>(null);
  const [loading, setLoading] = useState(true);
  const [toggling, setToggling] = useState(false);
  const [purging, setPurging] = useState(false);
  const [ttl, setTtl] = useState<number>(600);
  const [savingTtl, setSavingTtl] = useState(false);

  const fetchState = useCallback(async () => {
    setLoading(true);
    try {
      const res = await apiClient.get<CacheState>(`/domains/${domainId}/cache`);
      setState(res.data);
      setTtl(res.data.ttl_seconds ?? 600);
    } catch {
      message.error("Failed to load cache status");
    } finally {
      setLoading(false);
    }
  }, [domainId]);

  useEffect(() => {
    fetchState();
  }, [fetchState]);

  const onFlip = async (next: boolean) => {
    setToggling(true);
    try {
      const res = await apiClient.put<CacheState>(`/domains/${domainId}/cache`, {
        enabled: next,
      });
      setState(res.data);
      setTtl(res.data.ttl_seconds ?? 600);
      message.success(
        next
          ? "Caching enabled — applying to the site's nginx config"
          : "Caching disabled",
      );
    } catch {
      message.error("Failed to toggle caching");
      await fetchState();
    } finally {
      setToggling(false);
    }
  };

  const onSaveTtl = async () => {
    setSavingTtl(true);
    try {
      const res = await apiClient.put<CacheState>(`/domains/${domainId}/cache`, {
        enabled: !!state?.enabled,
        ttl_seconds: ttl,
      });
      setState(res.data);
      setTtl(res.data.ttl_seconds ?? 600);
      message.success(`Cache TTL set to ${res.data.ttl_seconds}s — re-rendering nginx`);
    } catch {
      message.error("Failed to update cache TTL");
      await fetchState();
    } finally {
      setSavingTtl(false);
    }
  };

  const onPurge = async () => {
    setPurging(true);
    try {
      await apiClient.post(`/domains/${domainId}/cache/purge`);
      message.success("Cache purged");
    } catch {
      message.error("Failed to purge cache");
    } finally {
      setPurging(false);
    }
  };

  if (loading) return <Skeleton active paragraph={{ rows: 1 }} />;

  const enabled = !!state?.enabled;
  const ttlDirty = enabled && ttl !== (state?.ttl_seconds ?? 600);

  return (
    <Space direction="vertical" style={{ width: "100%" }}>
      <Space size="middle" align="center" wrap>
        <Switch
          checkedChildren={<CheckOutlined />}
          unCheckedChildren={<CloseOutlined />}
          checked={enabled}
          loading={toggling}
          onChange={onFlip}
        />
        <span>nginx page cache</span>
        <Popconfirm
          title="Purge cached pages for this domain?"
          okText="Purge"
          onConfirm={onPurge}
          disabled={!enabled}
        >
          <Button
            icon={<ThunderboltOutlined />}
            loading={purging}
            disabled={!enabled}
          >
            Purge cache
          </Button>
        </Popconfirm>
      </Space>

      <Space size="small" align="center" wrap>
        <span>Cache TTL:</span>
        <InputNumber
          min={TTL_MIN}
          max={TTL_MAX}
          step={30}
          value={ttl}
          disabled={!enabled || savingTtl}
          onChange={(v) => setTtl(typeof v === "number" ? v : 600)}
          addonAfter="sec"
          style={{ width: 140 }}
        />
        <Button
          type="primary"
          size="small"
          loading={savingTtl}
          disabled={!ttlDirty}
          onClick={onSaveTtl}
        >
          Save TTL
        </Button>
      </Space>

      <Alert
        type="info"
        showIcon
        title="Page cache with background refresh"
        description="Caches PHP/HTML responses for the TTL above (default 600s). On expiry the stale page is served instantly while nginx refreshes it in the background — no visitor waits for the rebuild. Automatically bypassed for POST requests, logged-in WordPress users, carts/checkout, wp-admin and session cookies, so dynamic and authenticated pages stay correct."
      />
    </Space>
  );
};
