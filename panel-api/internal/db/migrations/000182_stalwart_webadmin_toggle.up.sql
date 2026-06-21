-- GH #243 (ADR-0142): opt-in exposure of the Stalwart WebAdmin UI through an
-- nginx reverse-proxy. Default OFF — Stalwart's admin listener stays pinned to
-- 127.0.0.1:8446 (M25 invariant); nginx is the only door, behind basic-auth +
-- TLS + an optional source-IP allowlist.
ALTER TABLE server_settings
  ADD COLUMN stalwart_webadmin_enabled     TINYINT(1)   NOT NULL DEFAULT 0,
  ADD COLUMN stalwart_webadmin_allow_cidrs VARCHAR(512) NOT NULL DEFAULT '';
