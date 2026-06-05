-- M48 Phase 1 (ADR-0116): docker_apps + docker_app_published_ports +
-- docker_app_backups tables. Schema only; the panel-api repo + reconciler
-- + REST handlers ship in later phases.

CREATE TABLE docker_apps (
  id              CHAR(26) NOT NULL,
  slug            VARCHAR(64) NOT NULL,
  name            VARCHAR(255) NOT NULL,
  catalog_version VARCHAR(64) NOT NULL,
  image_sha       VARCHAR(128) NULL,
  status          VARCHAR(32) NOT NULL DEFAULT 'pending',
  update_mode     VARCHAR(16) NOT NULL DEFAULT 'manual',
  cpu_limit       VARCHAR(16) NULL,
  memory_limit    VARCHAR(16) NULL,
  pids_limit      INT NULL,
  last_error      TEXT NULL,
  created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_docker_apps_slug_name (slug, name),
  KEY idx_docker_apps_status (status)
);

-- One row per container port the operator chose to publish. Catalog
-- declares all exposable ports; admin install drawer toggles each one.
-- See ADR-0116 Decision 5.
CREATE TABLE docker_app_published_ports (
  id              CHAR(26) NOT NULL,
  app_id          CHAR(26) NOT NULL,
  port_name       VARCHAR(32) NOT NULL,
  container_port  INT NOT NULL,
  bind_interface  VARCHAR(64) NOT NULL DEFAULT 'loopback',
  host_port       INT NOT NULL,
  protocol        VARCHAR(8) NOT NULL DEFAULT 'tcp',
  reverse_proxy   TINYINT(1) NOT NULL DEFAULT 1,
  enabled         TINYINT(1) NOT NULL DEFAULT 1,
  created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_dapp_pubport_per_app (app_id, port_name),
  UNIQUE KEY uniq_dapp_pubport_global (bind_interface, host_port, protocol),
  CONSTRAINT fk_dapp_pubport_app FOREIGN KEY (app_id) REFERENCES docker_apps(id) ON DELETE CASCADE
);

CREATE TABLE docker_app_backups (
  id              CHAR(26) NOT NULL,
  app_id          CHAR(26) NOT NULL,
  restic_id       VARCHAR(128) NOT NULL,
  size_bytes      BIGINT NOT NULL,
  reason          VARCHAR(32) NOT NULL DEFAULT 'manual',
  created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_dapp_backups_app (app_id),
  CONSTRAINT fk_dapp_backups_app FOREIGN KEY (app_id) REFERENCES docker_apps(id) ON DELETE CASCADE
);
