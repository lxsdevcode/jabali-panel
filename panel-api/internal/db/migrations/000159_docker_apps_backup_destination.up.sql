-- M48 Phase 8.2: each installed docker app can pick its own
-- backup destination from the M30 backup_destinations table. NULL
-- = fall back to the JABALI_RESTIC_REPO + JABALI_RESTIC_PASSWORD env
-- vars (Phase 8.1 behaviour).
ALTER TABLE docker_apps
  ADD COLUMN backup_destination_id CHAR(26) NULL,
  ADD CONSTRAINT fk_docker_apps_backup_dest
    FOREIGN KEY (backup_destination_id) REFERENCES backup_destinations(id) ON DELETE SET NULL;
