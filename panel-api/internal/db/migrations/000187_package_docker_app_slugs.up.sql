-- GH #170 #3: per-hosting-package Docker app allowlist. CSV of catalog slugs a
-- tenant on this package may install. Empty = fall back to the server-wide
-- docker_tenant_apps curation (backward-compatible). Gated additionally by
-- max_docker_apps (count) + tenant_installable (catalog).
ALTER TABLE hosting_packages
  ADD COLUMN docker_app_slugs VARCHAR(2000) NOT NULL DEFAULT '';
