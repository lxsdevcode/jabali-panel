DROP INDEX idx_users_disk_used_kb ON users;

ALTER TABLE users
  DROP COLUMN disk_used_kb,
  DROP COLUMN disk_limit_kb,
  DROP COLUMN disk_checked_at;
