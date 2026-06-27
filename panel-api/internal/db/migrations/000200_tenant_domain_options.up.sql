-- Tenant-safe domain options (GH #307). Opt-in: admins enable
-- tenant_domain_options_enabled to let non-admin owners set a curated, vetted
-- set of nginx options on their own domains (max body size, HSTS, security
-- headers, gzip) — rendered by the panel, never raw directives. Default OFF.
ALTER TABLE server_settings
  ADD COLUMN tenant_domain_options_enabled TINYINT(1) NOT NULL DEFAULT 0;
ALTER TABLE domains
  ADD COLUMN nginx_safe_options LONGTEXT NOT NULL DEFAULT '{}';
