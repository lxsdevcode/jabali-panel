-- GH #339 phase 1b: fuller PHP-FPM pool tuning knobs. Dynamic-mode process
-- manager sizing (start/min_spare/max_spare servers) + max_requests recycling +
-- request_terminate_timeout. Defaults are FPM-valid for the shipped ondemand
-- default (start/spare only take effect in dynamic mode); 0 = disabled for
-- max_requests + terminate timeout.
ALTER TABLE php_pools
  ADD COLUMN pm_start_servers                 INT UNSIGNED NOT NULL DEFAULT 2,
  ADD COLUMN pm_min_spare_servers             INT UNSIGNED NOT NULL DEFAULT 1,
  ADD COLUMN pm_max_spare_servers             INT UNSIGNED NOT NULL DEFAULT 3,
  ADD COLUMN pm_max_requests                  INT UNSIGNED NOT NULL DEFAULT 0,
  ADD COLUMN request_terminate_timeout_seconds INT UNSIGNED NOT NULL DEFAULT 0;
