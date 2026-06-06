-- M6.6 per-domain mail TLS — see plans/m6.6-per-domain-mail-tls.md
--
-- One row per tenant domain that has opted in to per-domain mail TLS.
-- The reconciler walks this table once per tick and dispatches
-- ssl.mail.issue / ssl.mail.renew through the agent. State machine
-- mirrors panel_certificate (ADR-0066, ADR-0105) on purpose so the
-- same status verbs work in both places.
--
-- domain_id is unique; opting in is a singleton-per-domain decision.
-- Cascade on domain delete so we don't widow rows when the operator
-- removes the domain.
CREATE TABLE mail_certificate (
  id              CHAR(26) PRIMARY KEY,
  domain_id       CHAR(26) NOT NULL,
  lineage_path    VARCHAR(255) NOT NULL DEFAULT '',
  status          VARCHAR(32) NOT NULL DEFAULT 'pending',
  last_error      TEXT NULL,
  attempt_count   INT UNSIGNED NOT NULL DEFAULT 0,
  next_retry_at   TIMESTAMP NULL,
  issued_at       TIMESTAMP NULL,
  expires_at      TIMESTAMP NULL,
  created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uq_mailcert_domain (domain_id),
  KEY ix_mailcert_status (status),
  KEY ix_mailcert_expires (expires_at),
  CONSTRAINT fk_mailcert_domain FOREIGN KEY (domain_id) REFERENCES domains(id) ON DELETE CASCADE
);
