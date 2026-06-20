-- GH #246: explicit per-domain TLS certificate mode.
--   le     — Let's Encrypt (ACME), self-signed fallback on failure (today's default)
--   self   — self-signed only, no ACME
--   custom — operator-supplied cert+key (no auto-renew)
--   none   — no certificate (HTTP only)
-- ssl_mode is authoritative; ssl_enabled is kept as a derived shadow
-- (ssl_mode != 'none') set on every write, to be dropped in a later migration.
ALTER TABLE domains
  ADD COLUMN ssl_mode ENUM('le','self','custom','none') NOT NULL DEFAULT 'le';
-- Backfill from the legacy boolean.
UPDATE domains SET ssl_mode = 'none' WHERE ssl_enabled = 0;
