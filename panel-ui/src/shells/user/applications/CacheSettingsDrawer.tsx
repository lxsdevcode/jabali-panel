// CacheSettingsDrawer — GH #616 + #612. Per-install cache tuning: page-cache URL
// exclusions (extra path prefixes that always bypass the nginx page cache, on
// top of the built-in wp-admin/cart/checkout set) PLUS the split behavioral
// controls (object-cache max-TTL, nginx page-cache TTL, Redis memory hint).
// Reads/writes GET/PUT /applications/:id/cache-settings; every value is
// panel-stamped as a JABALI_CACHE_* wp-config constant / domain setting by
// setCacheCore (the plugin never writes wp-config — GH #602). The reconciler
// re-renders the vhost and the agent re-stamps constants on save.
//
// A cookie bypass/allowlist was rejected: it would re-open the #416/#419
// fail-open class (cross-visitor content bleed on the shared microcache).
import { Alert, App, Drawer, Form, InputNumber, Select } from "antd";
import { useEffect, useState } from "react";

import { apiClient } from "../../../apiClient";
import { extractApiError } from "../../../apiErrors";
import { StandardDrawerFooter } from "../../../components/StandardActionFooter";

type CacheSettings = {
  url_exclusions?: string[];
  page_ttl?: number;
  object_maxttl?: number;
  redis_maxmemory_mb?: number;
};
type GetResp = {
  cache_enabled: boolean;
  configured: boolean;
  settings: CacheSettings;
};

export function CacheSettingsDrawer({
  install,
  onClose,
}: {
  install: { id: string; name?: string; cache_enabled?: boolean } | null;
  onClose: () => void;
}) {
  const { message } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [urlExclusions, setUrlExclusions] = useState<string[]>([]);
  const [pageTtl, setPageTtl] = useState<number | null>(null);
  const [objectMaxTtl, setObjectMaxTtl] = useState<number | null>(null);
  const [redisMaxMemory, setRedisMaxMemory] = useState<number | null>(null);

  useEffect(() => {
    if (!install) return;
    setLoading(true);
    apiClient
      .get<GetResp>(`/applications/${install.id}/cache-settings`)
      .then((res) => {
        const s = res.data.settings ?? {};
        setUrlExclusions(s.url_exclusions ?? []);
        setPageTtl(s.page_ttl ?? null);
        setObjectMaxTtl(s.object_maxttl ?? null);
        setRedisMaxMemory(s.redis_maxmemory_mb ?? null);
      })
      .catch((e) =>
        message.error(extractApiError(e) ?? "Failed to load cache settings"),
      )
      .finally(() => setLoading(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [install]);

  const save = async () => {
    if (!install) return;
    setSaving(true);
    try {
      await apiClient.put(`/applications/${install.id}/cache-settings`, {
        url_exclusions: urlExclusions,
        page_ttl: pageTtl ?? 0,
        object_maxttl: objectMaxTtl ?? 0,
        redis_maxmemory_mb: redisMaxMemory ?? 0,
      });
      message.success("Cache settings saved — applied on the next reconcile (~60s).");
      onClose();
    } catch (e) {
      message.error(extractApiError(e) ?? "Failed to save cache settings");
    } finally {
      setSaving(false);
    }
  };

  const enabled = install?.cache_enabled ?? false;

  return (
    <Drawer
      title={install ? `Cache settings — ${install.name ?? install.id}` : "Cache settings"}
      open={install !== null}
      onClose={onClose}
      width={520}
      footer={
        <StandardDrawerFooter
          primaryText="Save"
          primaryLoading={saving}
          onPrimary={() => void save()}
          onCancel={onClose}
        />
      }
    >
      {!enabled && (
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 16 }}
          message="Cache is off for this install"
          description="These settings are saved but only take effect once caching is enabled (the Cache toggle on the app row)."
        />
      )}
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="Applied on the next reconcile (~60s)"
        description="TTLs are stamped as wp-config constants / the domain page-cache TTL by the panel — the WordPress plugin never writes them. URL exclusions bypass the page cache on top of the built-in set (wp-admin, login, cart, checkout, my-account, the REST API)."
      />
      <Form layout="vertical" disabled={loading}>
        <Form.Item
          label="Page-cache TTL (seconds)"
          help="How long nginx serves a cached page before revalidating. Blank = domain default. Range 10-86400."
        >
          <InputNumber
            min={10}
            max={86400}
            value={pageTtl}
            onChange={(v) => setPageTtl(v)}
            placeholder="default"
            style={{ width: "100%" }}
          />
        </Form.Item>
        <Form.Item
          label="Object-cache max-TTL (seconds)"
          help="Cap on Redis object-cache key lifetime. 0 = no expiry (rely on Redis LRU)."
        >
          <InputNumber
            min={0}
            max={2592000}
            value={objectMaxTtl}
            onChange={(v) => setObjectMaxTtl(v)}
            placeholder="0 (LRU)"
            style={{ width: "100%" }}
          />
        </Form.Item>
        <Form.Item
          label="Redis memory hint (MB)"
          help="Advisory per-tenant Redis memory ceiling. 0 = server default. Enforcement is best-effort."
        >
          <InputNumber
            min={0}
            max={65536}
            value={redisMaxMemory}
            onChange={(v) => setRedisMaxMemory(v)}
            placeholder="0 (default)"
            style={{ width: "100%" }}
          />
        </Form.Item>
        <Form.Item
          label="URL exclusions"
          help="Path prefixes that must never be page-cached (e.g. /private). Enter to add."
        >
          <Select
            mode="tags"
            value={urlExclusions}
            onChange={setUrlExclusions}
            tokenSeparators={[",", " "]}
            placeholder="/private"
          />
        </Form.Item>
      </Form>
    </Drawer>
  );
}
