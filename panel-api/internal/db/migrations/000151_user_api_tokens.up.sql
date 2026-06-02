-- Per-user API tokens — programmatic access scoped to a single tenant.
--
-- Wire format: `Authorization: Bearer jat_<43-char-base64url>` (the "jat"
-- prefix mirrors "github personal access token" / "ghp_" style and makes
-- accidental commits to git pre-receive hooks easy to detect). We store
-- a sha256 of the FULL token (prefix included) in secret_hash + the
-- last 4 chars in secret_last4 for UI display ("jat_…aB9c"). Plaintext
-- token is returned once at mint and never persisted.
--
-- scopes_json is NULLABLE on this first cut: NULL = "full per-user
-- access" (the token can do anything its owner can do via the UI).
-- A scope picker (e.g. ["dns:write", "mailbox:read"]) lands in a
-- follow-up PR without needing a migration.
--
-- expires_at is NULLABLE: most tokens are infinite-lifetime API keys
-- the user rotates manually; short-lived tokens (e.g. CI runners with
-- 24h windows) opt in by setting expires_at.

CREATE TABLE user_api_tokens (
  id            CHAR(26)        NOT NULL,
  user_id       CHAR(26)        NOT NULL,
  name          VARCHAR(100)    NOT NULL,
  -- sha256 of the full bearer token (hex, lowercase). UNIQUE so a
  -- crafted-secret collision would 409 at mint instead of silently
  -- granting access to a different user's token.
  secret_hash   CHAR(64)        NOT NULL,
  -- Last 4 characters of the plaintext token, shown in the UI as a
  -- disambiguator (e.g. "jat_…aB9c"). Not security-sensitive.
  secret_last4  CHAR(4)         NOT NULL,
  scopes_json   JSON            NULL,
  last_used_at  DATETIME(6)     NULL,
  last_used_ip  VARCHAR(45)     NULL,
  expires_at    DATETIME(6)     NULL,
  revoked_at    DATETIME(6)     NULL,
  created_at    DATETIME(6)     NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uq_user_api_tokens_secret_hash (secret_hash),
  KEY        ix_user_api_tokens_user (user_id, revoked_at),
  CONSTRAINT fk_user_api_tokens_user
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
