-- GH #646: re-migration/refresh mode. 'import' = first migration / resume;
-- 'refresh' = force re-pull into an already-migrated account (backup-first).
ALTER TABLE migration_jobs
  ADD COLUMN mode VARCHAR(16) NOT NULL DEFAULT 'import';
