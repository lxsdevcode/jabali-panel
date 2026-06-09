-- M53 Updates Center. Turns the M29 thin-proxy updates page into a stateful
-- admin surface. Three tables, all schema-only here — the two singleton rows
-- (update_state id=1, update_autoupdate_config id=1) are seeded by the app on
-- first serve (EnsureDefault), never by this migration (migrations = schema,
-- data seeds live in the app — see the M24 000057 "Dirty database" scar).

-- Cheap page-load summary: last check results + counts, so the page renders
-- without an agent round-trip. Single row, id pinned to 1.
CREATE TABLE update_state (
  id                 TINYINT      NOT NULL DEFAULT 1,
  jabali_behind      INT          NOT NULL DEFAULT 0,
  jabali_current_sha CHAR(40)     NULL,
  jabali_checked_at  TIMESTAMP    NULL,
  apt_total          INT          NOT NULL DEFAULT 0,
  apt_security       INT          NOT NULL DEFAULT 0,
  apt_checked_at     TIMESTAMP    NULL,
  PRIMARY KEY (id),
  CONSTRAINT chk_update_state_single CHECK (id = 1)
);

-- One row per check/run; newest first powers the Recent History panel.
-- `unit` carries the transient systemd unit name so the run reconciler can
-- match a `running` row back to its host unit and mark it finished.
CREATE TABLE update_history (
  id             CHAR(26)     NOT NULL,
  kind           VARCHAR(16)  NOT NULL,            -- 'jabali' | 'apt'
  action         VARCHAR(16)  NOT NULL,            -- 'check' | 'run'
  status         VARCHAR(16)  NOT NULL,            -- 'success' | 'failed' | 'running'
  security_count INT          NOT NULL DEFAULT 0,
  summary        VARCHAR(255) NOT NULL DEFAULT '',
  log_excerpt    TEXT         NULL,
  unit           VARCHAR(128) NULL,
  started_at     TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  finished_at    TIMESTAMP    NULL,
  PRIMARY KEY (id),
  KEY idx_uh_started (started_at)
);

-- Desired auto-update state (DB-as-truth); the autoupdate reconciler converges
-- it onto the host (unattended-upgrades + jabali-autoupdate.timer). Single row.
CREATE TABLE update_autoupdate_config (
  id             TINYINT     NOT NULL DEFAULT 1,
  apt_enabled    BOOLEAN     NOT NULL DEFAULT FALSE,
  apt_time       VARCHAR(5)  NOT NULL DEFAULT '03:30',   -- HH:MM local
  jabali_enabled BOOLEAN     NOT NULL DEFAULT FALSE,
  jabali_time    VARCHAR(5)  NOT NULL DEFAULT '04:30',
  updated_at     TIMESTAMP   NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  CONSTRAINT chk_uac_single CHECK (id = 1)
);
