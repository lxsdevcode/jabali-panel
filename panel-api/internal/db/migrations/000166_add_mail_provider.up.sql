-- GH#181 / ADR-0120: per-domain mail provider.
-- mail_provider is the single source of truth for a domain's mail posture;
-- email_enabled + skip_auto_san are derived from it by the API on
-- create/edit. The columns below add the enum + the optional per-provider
-- DKIM tokens.
ALTER TABLE domains
  ADD COLUMN mail_provider VARCHAR(16) NOT NULL DEFAULT 'jabali',
  ADD COLUMN m365_onmicrosoft VARCHAR(255) NULL,
  ADD COLUMN google_dkim TEXT NULL;

-- Backfill (ADR-0120 carve-out):
--   email_enabled=0                       -> 'none'  (no mail records today)
--   email_enabled=1 AND skip_auto_san=0   -> 'jabali' (default; already set)
--   email_enabled=1 AND skip_auto_san=1   -> 'jabali' BUT skip_auto_san is
--     PRESERVED (the jabali=>skip_auto_san=false derivation fires only on an
--     explicit provider re-selection). No UPDATE touches skip_auto_san here,
--     so the ADR-0070 state survives.
UPDATE domains SET mail_provider = 'none' WHERE email_enabled = 0;

-- Widen dns_records.managed_by: the per-provider markers (mail-provider-m365
-- = 18 chars, mail-provider-google = 19) overflow the original VARCHAR(16).
ALTER TABLE dns_records MODIFY COLUMN managed_by VARCHAR(32) NULL;
