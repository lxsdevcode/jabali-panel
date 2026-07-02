-- GH #647: WordPress root path for wordpress_ssh migrations. Additive nullable
-- column; control-input, distinct from manifest_json (discoverer output).
-- (Coordination: #648's unmerged branch uses 000207 for migration_keys; whichever
-- lands second renumbers. #647 takes 207 here as the next free from main.)
ALTER TABLE migration_jobs ADD COLUMN source_path VARCHAR(1024) NULL;
