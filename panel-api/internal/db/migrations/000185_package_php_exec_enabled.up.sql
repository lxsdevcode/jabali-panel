-- GH #402: admin-only per-package opt-out from the #401 disable_functions
-- lockdown. When 1, pools owned by users on this package render WITHOUT the
-- command-exec blocklist (exec/proc_open/shell_exec/...). Default 0 keeps the
-- safe lockdown. Never tenant-settable — only an admin assigns packages.
ALTER TABLE hosting_packages
  ADD COLUMN php_exec_enabled TINYINT(1) NOT NULL DEFAULT 0;
