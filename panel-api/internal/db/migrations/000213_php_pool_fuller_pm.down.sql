ALTER TABLE php_pools
  DROP COLUMN pm_start_servers,
  DROP COLUMN pm_min_spare_servers,
  DROP COLUMN pm_max_spare_servers,
  DROP COLUMN pm_max_requests,
  DROP COLUMN request_terminate_timeout_seconds;
