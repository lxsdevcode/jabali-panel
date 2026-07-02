// CacheSettingsDrawer — GH #616. Per-install page-cache URL exclusions: extra
// path prefixes that always bypass the nginx page cache, on top of the built-in
// wp-admin/cart/checkout set. Reads/writes GET/PUT /applications/:id/cache-settings
// (envelope confirmed by applications_cache_settings_test.go); the reconciler
// merges these across a domain's installs and the agent re-sanitizes each before
// folding it into the bypass regex.
//
// Deliberately the ONLY control here. Object/page split + per-install TTL (#612)
// have no apply path yet (setCacheCore drives off cache_enabled; TTL is the
// per-domain domains.cache_ttl). Cache profiles (#618) are decorative — the
// built-in bypass set + the fail-closed cookie gate already cover the site types.
// A cookie bypass/allowlist was rejected: it would re-open the #416/#419
// fail-open class (cross-visitor content bleed on the shared microcache).
import { Alert, App, Drawer, Form, Select } from "antd";
import { useEffect, useState } from "react";

import { apiClient } from "../../../apiClient";
import { extractApiError } from "../../../apiErrors";
import { StandardDrawerFooter } from "../../../components/StandardActionFooter";

type CacheSettings = { url_exclusions?: string[] };
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

  useEffect(() => {
    if (!install) return;
    setLoading(true);
    apiClient
      .get<GetResp>(`/applications/${install.id}/cache-settings`)
      .then((res) => setUrlExclusions(res.data.settings?.url_exclusions ?? []))
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
      });
      message.success("Cache rules saved — nginx re-renders within ~60s.");
      onClose();
    } catch (e) {
      message.error(extractApiError(e) ?? "Failed to save cache settings");
    } finally {
      setSaving(false);
    }
  };

  return (
    <Drawer
      title={
        install
          ? `Cache rules — ${install.name ?? install.id}`
          : "Cache rules"
      }
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
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="Page-cache URL exclusions — applied on the next reconcile (~60s)"
        description="Paths listed here always bypass the page cache, in addition to the built-in exclusions (wp-admin, login, cart, checkout, my-account, the REST API, …). The cache on/off switch and page-cache TTL are managed elsewhere (the Cache toggle and the domain's cache settings)."
      />
      <Form layout="vertical" disabled={loading}>
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
