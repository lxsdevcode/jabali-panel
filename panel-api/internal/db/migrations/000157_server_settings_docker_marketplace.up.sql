-- M48 opt-in: docker app marketplace gated behind a server_settings
-- toggle. The /admin/docker-apps route group is unmounted when this
-- column is false. Mirrors the M37 postgres_enabled pattern: the
-- panel API dispatches an agent verb on flip that sources install.sh
-- and runs install_docker_engine (or removes the engine on flip
-- false, leaving customer data intact).
ALTER TABLE server_settings
  ADD COLUMN docker_marketplace_enabled TINYINT(1) NOT NULL DEFAULT 0;
