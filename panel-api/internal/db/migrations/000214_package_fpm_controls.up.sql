-- M PHP-FPM tiers Step 1: per-package FPM policy. Caps + opt-in gates + per-
-- version defaults that drive the tiered (L1 preset / L2 clamped / L3 admin)
-- pool tuning. Defaults keep every existing package at "tenants get nothing"
-- (both toggles 0) with a conservative 20-child cap.
ALTER TABLE hosting_packages
  ADD COLUMN fpm_max_children_cap  INT UNSIGNED  NOT NULL DEFAULT 20,
  ADD COLUMN fpm_worker_mem_mb     INT UNSIGNED  NOT NULL DEFAULT 64,
  ADD COLUMN fpm_user_can_edit     TINYINT(1)    NOT NULL DEFAULT 0,
  ADD COLUMN fpm_advanced_mode     TINYINT(1)    NOT NULL DEFAULT 0,
  ADD COLUMN fpm_version_defaults  VARCHAR(2000) NOT NULL DEFAULT '{}';
