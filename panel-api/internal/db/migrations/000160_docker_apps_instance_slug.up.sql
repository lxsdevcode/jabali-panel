-- M48 multi-install: per-install agent identifier.
--
-- Background. `slug` historically served two roles:
--   1. catalog reference (which template was this install rendered from)
--   2. on-disk identity (/var/lib/jabali/docker-apps/<slug>/ + container
--      name jabali-app-<slug>)
--
-- The dual role made it impossible to install the same catalog entry
-- twice (paths + container names collided). Split into two columns:
--
--   slug          = catalog reference (unchanged; resolves the template)
--   instance_slug = per-install identity used by the agent for paths,
--                   container names and restic tags
--
-- Backfill = existing rows keep their on-disk identity (instance_slug =
-- slug) so no agent state has to move on the host. New installs derive
-- instance_slug = "<catalog_slug>-<install_name>" so two installs of
-- the same catalog entry get distinct paths + containers.
ALTER TABLE docker_apps
  ADD COLUMN instance_slug VARCHAR(64) NOT NULL DEFAULT '';

UPDATE docker_apps SET instance_slug = slug WHERE instance_slug = '';
