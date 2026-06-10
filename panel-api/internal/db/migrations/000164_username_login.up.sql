-- M54 username-only login (ADR-0119). Username becomes the unique login
-- identifier; email is demoted to a non-unique contact so multiple accounts
-- may share one inbox.
--
-- Self-contained (touches only the users table), so the NULL-username backfill
-- runs safely inside this migration: the operator SHOULD run
-- `jabali admin backfill-usernames --apply` first for friendly email-derived
-- names, but this fallback guarantees no NULL/empty username survives to the
-- NOT NULL flip even if they don't. The fallback name is derived from the ULID
-- id (globally unique) so it can never collide. Valid per the schema pattern
-- (^[a-z_]...): 'user_' + lowercased 12-char id prefix.
UPDATE users
  SET username = CONCAT('user_', LOWER(SUBSTRING(id, 1, 12)))
  WHERE username IS NULL OR username = '';

-- Email may now repeat across accounts.
ALTER TABLE users DROP INDEX ux_users_email;

-- Username is the login identifier: required. (ux_users_username already
-- enforces uniqueness; this adds NOT NULL now that every row has one.)
ALTER TABLE users MODIFY username VARCHAR(32) NOT NULL;
