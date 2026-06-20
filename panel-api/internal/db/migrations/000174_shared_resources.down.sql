-- Reverse M52 shared-resources schema.
ALTER TABLE mail_groups DROP COLUMN group_kind;
DROP TABLE IF EXISTS shared_resource_grants;
DROP TABLE IF EXISTS shared_resources;
