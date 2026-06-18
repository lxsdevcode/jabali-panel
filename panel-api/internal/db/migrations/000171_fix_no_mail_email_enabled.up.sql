-- GH #216: correct email_enabled drift for non-jabali mail providers.
--
-- The Domain.EmailEnabled GORM field carried `default:1`, and GORM
-- substitutes a defaulted field's DB default for a Go zero value on
-- insert. So a domain created with mail_provider none/m365/google
-- (DeriveMailFlags → EmailEnabled=false) silently persisted
-- email_enabled=1. The reconciler then read it as mail-on and backfilled
-- the mail.<domain> certificate + mail DNS for "No Mail" domains.
--
-- The struct tag is dropped going forward (new rows write the real value).
-- This re-derives email_enabled from mail_provider — the single source of
-- truth (models.DeriveMailFlags) — for every already-drifted row, so the
-- mail-cert reconciler tears down the stale mail cert/DNS on its next pass.
-- jabali (and the default) stays mail-on; none/m365/google go mail-off.
UPDATE domains
SET email_enabled = 0
WHERE mail_provider IN ('none', 'm365', 'google')
  AND email_enabled = 1;
