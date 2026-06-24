-- Gitea #415: per-tenant salt for the WP-cache Redis ACL token. The token was a
-- pure function of (global secret, osuser) — deterministic, with no way to
-- rotate ONE tenant without rotating the global secret (which breaks every
-- tenant). A persisted per-user salt makes token = HMAC(secret, osuser|salt),
-- so a single tenant can be rotated by regenerating its salt.
CREATE TABLE cache_token_salts (
  user_id    CHAR(26)    NOT NULL PRIMARY KEY,
  salt       CHAR(64)    NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL
);
