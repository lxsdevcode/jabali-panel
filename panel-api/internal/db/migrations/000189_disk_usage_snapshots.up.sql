-- Per-user disk-usage snapshot. The tenant Disk Usage page reads the LAST
-- computed snapshot (a cheap DB read) and recomputes the quota/db.size/mailbox
-- aggregation ONLY on an explicit Refresh — so the page never auto-runs the
-- live agent calls on every visit.
CREATE TABLE disk_usage_snapshots (
  user_id     CHAR(26)    NOT NULL PRIMARY KEY,
  payload     LONGTEXT    NOT NULL,
  computed_at DATETIME(6) NOT NULL,
  created_at  DATETIME(6) NOT NULL,
  updated_at  DATETIME(6) NOT NULL
);
