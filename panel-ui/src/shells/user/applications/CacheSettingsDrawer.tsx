// CacheSettingsDrawer — GH #616/#618. Per-install page-cache RULES: a profile
// preset (Brochure / WooCommerce / Membership) plus URL-exclusion and
// cookie-bypass lists that layer onto the nginx page-cache gate at reconcile
// time. Reads/writes GET/PUT /applications/:id/cache-settings (envelope
// confirmed by applications_cache_settings_test.go).
//
// Deliberately NOT exposed here yet: the object/page cache split and a
// per-install TTL (#612). They have no apply path — setCacheCore drives
// provisioning off cache_enabled and the vhost TTL is the per-DOMAIN
// domains.cache_ttl (edited in DomainCacheSection). Shipping those switches now
// would paint controls that don't take effect; they land with #612 once the
// enable path consumes them. This drawer only exposes what the gate applies.
import { Alert, App, Drawer, Form, Select } from "antd";
import { useEffect, useState } from "react";

import { apiClient } from "../../../apiClient";
import { extractApiError } from "../../../apiErrors";
import { StandardDrawerFooter } from "../../../components/StandardActionFooter";

type CacheSettings = {
  profile?: string;
  url_exclusions?: string[];
  cookie_bypass?: string[];
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

  const [profile, setProfile] = useState<string>("");
  const [urlExclusions, setUrlExclusions] = useState<string[]>([]);
  const [cookieBypass, setCookieBypass] = useState<string[]>([]);

  useEffect(() => {
    if (!install) return;
    setLoading(true);
    apiClient
      .get<GetResp>(`/applications/${install.id}/cache-settings`)
      .then((res) => {
        const s = res.data.settings ?? {};
        setProfile(s.profile ?? "");
        setUrlExclusions(s.url_exclusions ?? []);
        setCookieBypass(s.cookie_bypass ?? []);
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
        profile: profile || undefined,
        url_exclusions: urlExclusions,
        cookie_bypass: cookieBypass,
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
        install ? `Cache rules — ${install.name ?? install.id}` : "Cache rules"
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
        message="Page-cache rules — applied on the next reconcile (~60s)"
        description="A profile seeds safe defaults for the site type; URL exclusions and cookie bypass layer on top of the built-in gate. The cache on/off switch and page-cache TTL are managed elsewhere (the Cache toggle and the domain's cache settings)."
      />
      <Form layout="vertical" disabled={loading}>
        <Form.Item
          label="Cache profile"
          help="Seeds safe page-cache rules for the site type."
        >
          <Select
            value={profile}
            onChange={setProfile}
            options={[
              { value: "", label: "Brochure (default)" },
              { value: "woocommerce", label: "WooCommerce" },
              { value: "membership", label: "Membership / LMS" },
            ]}
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
        <Form.Item
          label="Cookie bypass"
          help="Cookie name prefixes that force a page-cache bypass. Enter to add."
        >
          <Select
            mode="tags"
            value={cookieBypass}
            onChange={setCookieBypass}
            tokenSeparators={[",", " "]}
            placeholder="my_session_"
          />
        </Form.Item>
      </Form>
    </Drawer>
  );
}
