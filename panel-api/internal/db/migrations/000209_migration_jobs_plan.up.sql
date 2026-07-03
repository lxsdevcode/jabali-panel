-- GH #665: per-migration plan (account selection + import-area toggles) as JSON.
-- Additive nullable; NULL = import everything (current behavior).
ALTER TABLE migration_jobs ADD COLUMN plan_json TEXT NULL;
