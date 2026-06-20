-- GH #237: per-external-forward "keep a copy in this mailbox" toggle.
-- DEFAULT 0 = Microsoft 365-style forwarding (forward only, no local copy)
-- unless the user explicitly ticks "Keep a copy". The API always sets this
-- column explicitly on create; alias rows ignore it.
ALTER TABLE email_forwarders
  ADD COLUMN keep_copy TINYINT(1) NOT NULL DEFAULT 0;
