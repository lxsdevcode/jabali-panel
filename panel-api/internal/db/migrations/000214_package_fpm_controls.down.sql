ALTER TABLE hosting_packages
  DROP COLUMN fpm_max_children_cap,
  DROP COLUMN fpm_worker_mem_mb,
  DROP COLUMN fpm_user_can_edit,
  DROP COLUMN fpm_advanced_mode,
  DROP COLUMN fpm_version_defaults;
