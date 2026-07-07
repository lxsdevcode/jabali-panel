-- M PHP-FPM tiers Step 2: the 4 admin-editable Performance Mode presets that
-- L1 users pick from. Schema only — the 4 default rows are seeded app-side on
-- first boot (PHPPerformanceModeRepository.EnsureDefaults), never from the
-- migration (migrations = schema only, per the dirty-DB scar).
CREATE TABLE php_performance_modes (
  mode                              VARCHAR(24)  NOT NULL PRIMARY KEY,
  label                             VARCHAR(64)  NOT NULL DEFAULT '',
  pm_mode                           VARCHAR(16)  NOT NULL DEFAULT 'ondemand',
  pm_max_children                   INT UNSIGNED NOT NULL DEFAULT 10,
  pm_start_servers                  INT UNSIGNED NOT NULL DEFAULT 2,
  pm_min_spare_servers              INT UNSIGNED NOT NULL DEFAULT 1,
  pm_max_spare_servers              INT UNSIGNED NOT NULL DEFAULT 3,
  pm_max_requests                   INT UNSIGNED NOT NULL DEFAULT 0,
  request_terminate_timeout_seconds INT UNSIGNED NOT NULL DEFAULT 0,
  process_idle_timeout_seconds      INT UNSIGNED NOT NULL DEFAULT 60,
  updated_at                        DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
