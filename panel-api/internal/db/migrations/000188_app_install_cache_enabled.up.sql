-- GH #406: per-WordPress-app caching toggle (jabali-wp-cache plugin + nginx page cache).
ALTER TABLE application_installs
  ADD COLUMN cache_enabled TINYINT(1) NOT NULL DEFAULT 0;
