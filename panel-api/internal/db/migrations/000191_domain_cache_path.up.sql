-- Gitea #420: scope the per-domain page cache to the WordPress install's path.
-- A domain can host a WP install at /blog AND other (non-WP, differently-authed)
-- content elsewhere; the WP-tuned cache + skip rules must not apply to the whole
-- server block. cache_path is the install's path prefix ("/" = whole domain,
-- the back-compatible default for single-app domains).
ALTER TABLE domains
  ADD COLUMN cache_path VARCHAR(255) NOT NULL DEFAULT '/';
