ALTER TABLE docker_apps DROP FOREIGN KEY fk_docker_apps_backup_dest;
ALTER TABLE docker_apps DROP COLUMN backup_destination_id;
