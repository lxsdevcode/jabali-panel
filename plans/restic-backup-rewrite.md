# Plan: Restic Backup System Rewrite

> Source PRD: Complete rewrite of backup pages, services, and agent integration to use Restic natively. Clean break — no legacy tar.gz support.

## Architectural decisions

- **Restic is the only backup engine.** No tar.gz, no rsync --link-dest, no pigz. All backups are Restic snapshots.
- **Every backup is incremental by design.** Restic deduplicates at the block level. No "full vs incremental" distinction in the UI.
- **Agent wraps Restic CLI.** The agent runs as root and executes `restic backup`, `restic restore`, `restic snapshots`, `restic forget`, `restic ls`, `restic check`. The panel never calls Restic directly.
- **Destination config includes auth.** The agent receives full destination config (SFTP password, S3 keys) so it can construct the proper Restic environment. The panel passes `destination` config array to every agent call.
- **Restic password:** Auto-generated per server at `/etc/jabali/restic-password`. One password for all repos on this server.
- **Routes:** Admin backup page at `/jabali-admin/backups`. User backup page at `/jabali-panel/backups`. No separate restore pages — restore is inline within the backup page.
- **Models stay:** `Backup`, `BackupDestination`, `BackupSchedule`, `BackupRestore`, `UserRemoteBackup` — fields may change but models remain.
- **Schema changes:** `Backup.snapshot_id` (already added). Remove `Backup.type` enum values `incremental`/`partial` — all backups are `restic`. Remove `Backup.local_path`/`Backup.checksum` (Restic manages storage). `BackupDestination` drops `nfs` type — only `local`, `sftp`, `s3`.
- **No legacy code.** All tar.gz listing, content browsing, rsync upload, and dirvish-style incremental functions are removed.

---

## Phase 1: Destination Management (Create + Test)

**User stories:** Admin can add an SFTP or S3 backup destination, test the connection, and see the result. User can add their own SFTP destination.

### What to build

A clean Destinations tab on both admin and user backup pages. The admin page shows server-level destinations; the user page shows user-scoped destinations. Each destination has a name, type (local/sftp/s3), and config fields. "Test Connection" calls the agent's `backup.test_destination` which runs `restic init` or `restic snapshots` to verify connectivity. Results are shown as success/failure with a timestamp.

### Acceptance criteria

- [ ] Admin can create a local, SFTP, or S3 destination with proper form fields
- [ ] Admin can test a destination and see success/failure notification
- [ ] Admin can edit and delete destinations
- [ ] User can create/test/delete their own SFTP destination
- [ ] `BackupDestination.getResticRepoUrl()` returns correct repo URL for each type
- [ ] Agent `backup.test_destination` works with SFTP password auth (via sshpass)
- [ ] Agent `backup.test_destination` works with S3 credentials

---

## Phase 2: Create Backup (Admin + User)

**User stories:** Admin can trigger a server backup for all/selected users to any destination. User can trigger a personal backup. Backup progress is visible.

### What to build

"Create Server Backup" button on admin page opens a modal: select destination, select users (or all), toggle what to include (files/databases/mailboxes/DNS/SSL). Submitting dispatches `RunServerBackup` job. The job calls `BackupOrchestrator.execute()` which calls the agent's `backup.create_server`. The agent stages data (mysqldump, file paths, DNS exports, SSL certs) and runs `restic backup` with `--tag user:{username}` and `--json`. Snapshot ID is stored in the Backup model. User page has a simpler "Create Backup" that backs up their own data.

### Acceptance criteria

- [ ] Admin can create a server backup targeting specific users and destination
- [ ] User can create a personal backup to local or their SFTP destination
- [ ] Backup record shows status progression: pending → running → completed/failed
- [ ] Snapshot ID is stored on successful backup
- [ ] Backup size is captured from Restic summary
- [ ] Failed backups show error message
- [ ] `IndexRemoteBackups` job dispatches after completion to index new snapshots

---

## Phase 3: List Snapshots + Delete

**User stories:** Admin sees all backups with status, size, destination, and age. User sees their own. Either can delete a snapshot.

### What to build

Snapshots/Backups tab shows a Filament table of Backup records. Columns: name, status (badge), size (human-readable), destination name, created date, duration. Delete action calls `BackupOrchestrator.deleteBackup()` which runs `restic forget {snapshot_id} --prune` via the agent. Confirmation dialog required.

### Acceptance criteria

- [ ] Admin backup table shows all server + user backups
- [ ] User backup table shows only their backups
- [ ] Delete removes the Restic snapshot AND the DB record
- [ ] Deleted snapshots that fail to prune show a warning but still delete the DB record
- [ ] Table supports search and pagination

---

## Phase 4: Restore (Full)

**User stories:** Admin can restore a backup for any user. User can restore their own backup. Full restore includes files, databases, and mailboxes.

### What to build

Restore action on a backup row opens a modal. For admin: select which user to restore (from snapshot tags). Toggles for files/databases/mailboxes. Submit calls the agent's `backup.restore` with the snapshot ID. The agent runs `restic restore` to a temp directory, then applies the data (rsync files, mysql import, copy mailboxes, fix ownership). A `BackupRestore` record tracks the operation.

### Acceptance criteria

- [ ] Admin can restore any backup for any user
- [ ] User can restore their own backup
- [ ] Restore toggles: files, databases, mailboxes
- [ ] BackupRestore record created with status tracking
- [ ] Success notification shows counts (domains, databases, mailboxes restored)
- [ ] Failed restore shows error message
- [ ] File ownership is fixed after restore

---

## Phase 5: Scheduled Backups + Retention

**User stories:** Admin can schedule daily/weekly/monthly backups with retention policies. Schedules run automatically via cron.

### What to build

Schedules tab on admin page. Create/edit schedule with: name, frequency (daily/weekly/monthly), time, destination, users, retention count. `RunBackupSchedules` artisan command (already scheduled via cron) finds due schedules, creates Backup records, and dispatches jobs. After backup completes, `BackupOrchestrator.applyRetention()` runs `restic forget --keep-last {N} --prune`.

### Acceptance criteria

- [ ] Admin can create/edit/delete/toggle backup schedules
- [ ] Schedules show last run time and status
- [ ] Cron triggers due schedules automatically
- [ ] Retention policy prunes old snapshots via Restic forget
- [ ] Old Backup DB records beyond retention are cleaned up

---

## Phase 6: Browse Snapshot Contents + Selective Restore

**User stories:** User can browse what's inside a snapshot before restoring. User can restore specific domains, databases, or mailboxes instead of everything.

### What to build

"Browse" action on a snapshot calls `restic ls {snapshot_id} --json` via the agent to list contents. Results are categorized (domains, databases, mailboxes, DNS zones, SSL certs). User selects specific items to restore. Submit calls `GranularRestoreService.selectiveRestore()` which passes `--include` paths to `restic restore`. Conflict resolution options: overwrite, merge, skip.

### Acceptance criteria

- [ ] User can browse snapshot contents categorized by type
- [ ] User can select specific domains/databases/mailboxes to restore
- [ ] Selective restore only restores selected items
- [ ] Conflict resolution (overwrite/skip) is respected
- [ ] Domain file browser allows drilling into subdirectories

---

## Phase 7: Backup Logs + Verification

**User stories:** Admin can view backup logs for troubleshooting. Admin can verify backup integrity.

### What to build

Logs tab shows backup-related log entries from Laravel log filtered by backup keywords. "Verify" action on a destination runs `restic check --repo {url}` via the agent and reports integrity status. Admin notification on backup failure (already wired via `AdminNotificationService`).

### Acceptance criteria

- [ ] Logs tab shows recent backup-related log entries
- [ ] Log entries are filterable by date and keyword
- [ ] "Verify Repository" action runs restic check and shows result
- [ ] Failed backups trigger admin notification
- [ ] Backup schedule failures are logged with error details
