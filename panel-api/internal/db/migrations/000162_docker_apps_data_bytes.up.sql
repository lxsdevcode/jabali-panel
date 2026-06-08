-- M48 disk usage: persistent-data footprint per install, shown in the
-- admin Docker Apps list so operators can see what each install costs on
-- disk (the data dir under /var/lib/jabali/docker-apps/<instance_slug>/
-- is what grows over time and fills the host).
--
-- Harvested by the reconciler's status poll on a slow cadence (not every
-- tick — `du` walks the tree, so it's gated to ~15m via size_checked_at)
-- and written back here; the list endpoint reads the row.
ALTER TABLE docker_apps
  ADD COLUMN data_bytes BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN size_checked_at TIMESTAMP NULL;
