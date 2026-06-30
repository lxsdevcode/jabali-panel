-- Gitea #596: per-domain page-cache TTL (seconds). Default 600s — heavy pages
-- should cache for minutes, not the old hardcoded 60s. fastcgi_cache_background_update
-- (added to the vhost template) keeps freshness without blocking visitors.
ALTER TABLE domains
  ADD COLUMN cache_ttl_seconds INT NOT NULL DEFAULT 600;
