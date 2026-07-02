-- GH #647/#648: destination for auto-import. When set, the pull auto-chains to
-- migration.import_wp_run on success, so a migration is a fire-and-forget
-- background job (no need to keep a UI drawer open).
ALTER TABLE migration_jobs ADD COLUMN dest_user VARCHAR(64) NULL;
ALTER TABLE migration_jobs ADD COLUMN dest_domain VARCHAR(255) NULL;
