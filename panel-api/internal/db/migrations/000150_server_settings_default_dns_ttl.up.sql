-- Server-wide default TTL applied to newly-created DNS records when
-- the API caller doesn't specify one explicitly. Replaces the hardcoded
-- 3600 fallback in panel-api/internal/api/dns.go createRecord. Range
-- 60-86400 (1 minute to 1 day) is enforced at the API layer; column
-- default 3600 matches the pre-2026-06 hardcoded fallback so existing
-- installs see no behavior change until the admin saves a new value.
ALTER TABLE server_settings
  ADD COLUMN default_dns_ttl INT UNSIGNED NOT NULL DEFAULT 3600
  AFTER admin_email;
