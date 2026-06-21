-- Reverse M49 Phase 1.
ALTER TABLE hosting_packages
  DROP COLUMN max_docker_apps;

ALTER TABLE docker_apps
  DROP KEY uniq_docker_apps_owner_slug_name,
  ADD UNIQUE KEY uniq_docker_apps_slug_name (slug, name);

ALTER TABLE docker_apps
  DROP FOREIGN KEY fk_docker_apps_user,
  DROP KEY idx_docker_apps_user,
  DROP COLUMN user_id;
