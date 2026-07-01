-- GH #598: auto-allowlist successful panel + SSH login source IPs in CrowdSec.
-- Enabled by default (matches the already-shipped panel-login behaviour); TTL
-- in hours, default 168h (7 days), refreshed on activity. Admin-configurable.
ALTER TABLE server_settings
  ADD COLUMN crowdsec_login_allowlist_enabled   BOOLEAN NOT NULL DEFAULT TRUE,
  ADD COLUMN crowdsec_login_allowlist_ttl_hours INT     NOT NULL DEFAULT 168;
