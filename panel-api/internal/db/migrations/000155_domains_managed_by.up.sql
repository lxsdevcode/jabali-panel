-- M48 Phase 1 (ADR-0116 Decision 4): tag domains that the docker-app
-- subsystem auto-creates so the tenant-side CRUD path and the docker-app
-- CRUD path stay separate.
--
-- managed_by values:
--   tenant     (default) — created via the existing /domains handler.
--   docker_app — created by the docker-app installer; auto-managed.
--
-- docker_app_id back-references docker_apps.id when managed_by='docker_app'.
-- Reconciler uses this to render a proxy_pass vhost to the app's loopback
-- port (set in docker_app_published_ports for reverse_proxy=true rows).

ALTER TABLE domains
  ADD COLUMN managed_by    VARCHAR(32) NOT NULL DEFAULT 'tenant',
  ADD COLUMN docker_app_id CHAR(26) NULL,
  ADD CONSTRAINT fk_domains_docker_app FOREIGN KEY (docker_app_id) REFERENCES docker_apps(id) ON DELETE SET NULL,
  ADD KEY idx_domains_managed_by (managed_by);
