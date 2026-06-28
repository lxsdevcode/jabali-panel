-- Per-package Python app entitlement + quota (Gitea #491). Mirrors
-- max_docker_apps: 0 = Python apps NOT included (safe default, opt-in per plan).
ALTER TABLE hosting_packages
  ADD COLUMN max_python_apps INT UNSIGNED NOT NULL DEFAULT 0;
