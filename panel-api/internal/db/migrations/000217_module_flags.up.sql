-- M353 Phase 1 (GH #353): per-module enable flags so an operator can run only
-- the pieces they need + the panel hides the disabled pages. DEFAULT 1 so
-- upgrades of existing installs keep every module on (no silent feature loss);
-- fresh installs get the flags seeded from JABALI_MODULES at first boot (app
-- seed, not this migration — migrations are schema only). postgres_enabled
-- already exists from an earlier migration.
ALTER TABLE server_settings
  ADD COLUMN dns_enabled      BOOLEAN NOT NULL DEFAULT 1,
  ADD COLUMN mail_enabled     BOOLEAN NOT NULL DEFAULT 1,
  ADD COLUMN security_enabled BOOLEAN NOT NULL DEFAULT 1,
  ADD COLUMN quota_enabled    BOOLEAN NOT NULL DEFAULT 1,
  ADD COLUMN api_enabled      BOOLEAN NOT NULL DEFAULT 1;
