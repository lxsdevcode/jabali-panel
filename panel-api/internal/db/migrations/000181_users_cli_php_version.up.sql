-- GH #256: per-user CLI default PHP version. The user's bare `php` (and
-- composer/wp-cli) resolves to this version when set; NULL = auto (follow the
-- pool-derived pin). Independent of the per-domain web PHP version, since one
-- user spans many domains on different versions. The agent's user-phpcli choice
-- file is the host-side source of truth; this column is the panel's record so
-- the UI can show + edit it.
ALTER TABLE users ADD COLUMN cli_php_version VARCHAR(8) NULL;
