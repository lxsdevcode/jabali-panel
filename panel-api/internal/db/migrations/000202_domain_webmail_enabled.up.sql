-- Per-domain webmail toggle (GH #316): disable the Bulwark webmail vhost for a
-- specific domain while keeping mail delivery. Default 1 = ON (no regression;
-- existing email domains keep webmail). AND-ed with the global webmail_enabled.
ALTER TABLE domains ADD COLUMN webmail_enabled TINYINT(1) NOT NULL DEFAULT 1;
