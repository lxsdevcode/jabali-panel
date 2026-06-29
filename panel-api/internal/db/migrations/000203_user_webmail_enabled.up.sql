-- Per-user webmail toggle (GH #316): disable the Bulwark webmail vhost for ALL
-- of a user's domains while keeping mail delivery. AND-ed with the global
-- (server_settings.webmail_enabled) and per-domain (domains.webmail_enabled)
-- flags in the webmail reconciler. Default 1 = ON (no regression; existing
-- users keep webmail).
ALTER TABLE users ADD COLUMN webmail_enabled TINYINT(1) NOT NULL DEFAULT 1;
