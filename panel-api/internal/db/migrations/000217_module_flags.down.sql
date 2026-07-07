ALTER TABLE server_settings
  DROP COLUMN dns_enabled,
  DROP COLUMN mail_enabled,
  DROP COLUMN security_enabled,
  DROP COLUMN quota_enabled,
  DROP COLUMN api_enabled;
