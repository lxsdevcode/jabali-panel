-- M49 Phase 1 (ADR-0117, GH #170): tenant-owned docker apps.
--
-- user_id NULL  = admin / server-level install (the M48 behaviour, preserved).
-- user_id set   = tenant-owned; every tenant verb filters on it; a user delete
--                 cascades the row (the delete path enqueues host teardown of
--                 the container + data tree before the cascade fires).
ALTER TABLE docker_apps
  ADD COLUMN user_id CHAR(26) NULL AFTER id,
  ADD CONSTRAINT fk_docker_apps_user
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  ADD KEY idx_docker_apps_user (user_id);

-- Cross-tenant collision fix. M48's uniq_docker_apps_slug_name (slug, name) is
-- global, so two different tenants could not both install e.g. "memos" called
-- "notes" — the row, the container name (jabali-app-<instance_slug>) and the
-- data dir would collide. Namespace uniqueness by owner. NULL user_id rows
-- (admin installs) are treated as distinct by MySQL/MariaDB in a unique key, so
-- admin-install slug+name uniqueness is kept by the app-layer FindBySlugName
-- check (matching the pre-M49 behaviour).
ALTER TABLE docker_apps
  DROP KEY uniq_docker_apps_slug_name,
  ADD UNIQUE KEY uniq_docker_apps_owner_slug_name (user_id, slug, name);

-- Package gate: 0 = docker apps NOT included in this package (safe default —
-- opt-in per plan, no surprise capability grant on existing packages).
-- >0 = max simultaneous tenant installs for users on the plan.
ALTER TABLE hosting_packages
  ADD COLUMN max_docker_apps INT UNSIGNED NOT NULL DEFAULT 0 AFTER max_tasks;
