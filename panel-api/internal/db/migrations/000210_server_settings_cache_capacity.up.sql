-- GH #634: configurable shared FastCGI cache capacity.
ALTER TABLE server_settings
  ADD COLUMN nginx_cache_max_size_gb INT NOT NULL DEFAULT 4,
  ADD COLUMN nginx_cache_keyzone_mb INT NOT NULL DEFAULT 64,
  ADD COLUMN nginx_cache_inactive_min INT NOT NULL DEFAULT 60;
