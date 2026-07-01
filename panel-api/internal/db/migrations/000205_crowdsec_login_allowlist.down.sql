ALTER TABLE server_settings
  DROP COLUMN crowdsec_login_allowlist_enabled,
  DROP COLUMN crowdsec_login_allowlist_ttl_hours;
