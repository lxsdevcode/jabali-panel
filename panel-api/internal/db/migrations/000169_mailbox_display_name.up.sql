ALTER TABLE mailboxes
  ADD COLUMN display_name VARCHAR(255) NOT NULL DEFAULT '' AFTER email_cached;
