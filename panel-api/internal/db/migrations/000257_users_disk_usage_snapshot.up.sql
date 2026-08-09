-- Persist each account's disk usage on the users row so the admin Users
-- list can ORDER BY it.
--
-- Before this, the Disk usage column had no sorter and could not get one:
-- sorting is server-side against a per-repo column whitelist, and the
-- number was not in the database at all. Every row fetched its own
-- /users/:id/usage (agent `quota` call) after render, so an 80-row page
-- issued 80 lazy requests and the table never held the values it would
-- have had to sort.
--
-- Written by the disk-usage sweeper (panel-api/internal/diskusagesweeper),
-- which reuses the same agent report the per-row endpoint calls, so the
-- persisted figure matches what the detail view shows.
--
-- disk_checked_at NULL = never swept; the UI falls back to the per-row
-- fetch for those so a fresh upgrade shows numbers before the first sweep.
ALTER TABLE users
  ADD COLUMN disk_used_kb BIGINT UNSIGNED NOT NULL DEFAULT 0,
  ADD COLUMN disk_limit_kb BIGINT UNSIGNED NOT NULL DEFAULT 0,
  ADD COLUMN disk_checked_at DATETIME(6) NULL;

-- Sorting the Users list is the whole point; without this the ORDER BY is
-- a filesort over every account.
CREATE INDEX idx_users_disk_used_kb ON users (disk_used_kb);
