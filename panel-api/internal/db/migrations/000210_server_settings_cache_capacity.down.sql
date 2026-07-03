ALTER TABLE server_settings
  DROP COLUMN nginx_cache_max_size_gb,
  DROP COLUMN nginx_cache_keyzone_mb,
  DROP COLUMN nginx_cache_inactive_min;
