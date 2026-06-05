ALTER TABLE domains DROP FOREIGN KEY fk_domains_docker_app;
ALTER TABLE domains DROP KEY idx_domains_managed_by;
ALTER TABLE domains DROP COLUMN docker_app_id;
ALTER TABLE domains DROP COLUMN managed_by;
