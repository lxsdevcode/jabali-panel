-- JAB-140: write-automation layer on the M44 Automation API.
--
-- Additive + schema-only (migration-data-seed-ordering scar). Existing read
-- tokens are unaffected: writes_enabled only gates write scopes, and
-- ip_allowlist_json / expires_at are NULL (= no restriction) for legacy rows.
--
-- A write-scoped automation token is a headless, server-wide admin key, so the
-- token row carries extra guards independent of its scopes.
ALTER TABLE automation_tokens
  ADD COLUMN writes_enabled    TINYINT(1) NOT NULL DEFAULT 1,
  ADD COLUMN ip_allowlist_json JSON       NULL,
  ADD COLUMN expires_at        DATETIME(6) NULL;

-- automation_operations: async-action tracker (backups) + idempotency ledger.
-- Fast writes complete synchronously and never insert here. No FK to
-- automation_tokens (tokens are revocable; keep the op history).
CREATE TABLE automation_operations (
  id              CHAR(26)     NOT NULL PRIMARY KEY,
  token_id        CHAR(26)     NOT NULL,
  idempotency_key VARCHAR(80)  NULL,
  action          VARCHAR(64)  NOT NULL,
  target          VARCHAR(255) NOT NULL DEFAULT '',
  status          ENUM('pending','running','done','error') NOT NULL DEFAULT 'pending',
  message         VARCHAR(512) NOT NULL DEFAULT '',
  created_at      DATETIME(6)  NOT NULL,
  updated_at      DATETIME(6)  NOT NULL,
  KEY ix_ao_token (token_id),
  UNIQUE KEY uq_ao_idem (token_id, idempotency_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
