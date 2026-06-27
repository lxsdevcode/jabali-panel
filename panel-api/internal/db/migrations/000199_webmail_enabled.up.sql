-- Webmail (Bulwark) enable/disable toggle (GH #316). Default ON to preserve
-- current behavior; when off, the reconciler tears down the webmail vhosts and
-- stops the jabali-webmail service.
ALTER TABLE server_settings
  ADD COLUMN webmail_enabled TINYINT(1) NOT NULL DEFAULT 1;
