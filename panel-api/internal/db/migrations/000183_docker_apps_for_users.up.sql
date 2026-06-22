-- Admin opt-in to expose tenant Docker Apps to users (separate from the
-- server-wide docker_marketplace_enabled, which only installs the engine).
-- Default OFF — even with the engine installed, users see no Docker Apps tab
-- until the admin enables it (and the host is made tenant-ready via
-- `jabali docker enable-tenant`).
ALTER TABLE server_settings
  ADD COLUMN docker_apps_for_users_enabled TINYINT(1) NOT NULL DEFAULT 0;
