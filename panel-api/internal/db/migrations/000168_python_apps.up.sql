-- M-Python (ADR-0131 / GH #203): native Python app hosting.
-- python_apps + python_app_env registry; server_settings opt-in flag.
-- Schema only; agent/reconciler/API/UI ship in later waves.

CREATE TABLE python_apps (
  id             CHAR(26) NOT NULL,
  user_id        CHAR(26) NOT NULL,
  domain_id      CHAR(26) NOT NULL,
  name           VARCHAR(255) NOT NULL,
  runtime        VARCHAR(16) NOT NULL DEFAULT 'python',
  python_version VARCHAR(16) NOT NULL,
  app_root       VARCHAR(512) NOT NULL,
  app_type       VARCHAR(8) NOT NULL DEFAULT 'wsgi',
  entrypoint     VARCHAR(255) NOT NULL,
  base_uri       VARCHAR(255) NOT NULL DEFAULT '/',
  loopback_port  INT NULL,
  start_command  VARCHAR(1024) NULL,
  status         VARCHAR(32) NOT NULL DEFAULT 'pending',
  cpu_limit      VARCHAR(16) NULL,
  memory_limit   VARCHAR(16) NULL,
  pids_limit     INT NULL,
  last_error     TEXT NULL,
  created_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  -- One app per (domain, mount point); two apps can't both own /app.
  UNIQUE KEY uniq_python_apps_domain_base (domain_id, base_uri),
  KEY idx_python_apps_user (user_id),
  KEY idx_python_apps_domain (domain_id),
  KEY idx_python_apps_status (status),
  UNIQUE KEY uniq_python_apps_port (loopback_port)
);

-- Per-app environment variables (EnvironmentFile for the systemd unit).
CREATE TABLE python_app_env (
  id         CHAR(26) NOT NULL,
  app_id     CHAR(26) NOT NULL,
  env_key    VARCHAR(255) NOT NULL,
  env_value  TEXT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_python_app_env_key (app_id, env_key),
  KEY idx_python_app_env_app (app_id)
);

-- Opt-in: hidden until an admin enables it in Server Settings -> Apps.
ALTER TABLE server_settings
  ADD COLUMN python_apps_enabled TINYINT(1) NOT NULL DEFAULT 0;
