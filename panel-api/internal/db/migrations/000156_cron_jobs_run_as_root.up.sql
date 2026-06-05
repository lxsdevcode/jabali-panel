-- Add run_as_root flag to cron_jobs. Admin-only at the API gate
-- (panel-api/internal/api/cron.go honours the field only when
-- claims.IsAdmin). The agent branches on this column to write a
-- system-scoped systemd timer at /etc/systemd/system/ instead of a
-- per-user one at /etc/jabali-panel/cron-units/<user>/.
--
-- user_id stays the OWNER of the row (the admin who created it).
-- Execution context is independent: when run_as_root=1 the command
-- runs as root regardless of which admin owns the row.
ALTER TABLE cron_jobs
  ADD COLUMN run_as_root TINYINT(1) NOT NULL DEFAULT 0;
