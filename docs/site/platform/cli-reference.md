# Jabali CLI reference

> Generated from the Cobra command tree — do not edit by hand.
> Regenerate with `go test ./panel-api/cmd/server -run TestCLIReferenceGolden -update`.

## `jabali`

Jabali Panel — web hosting control panel

```
jabali
```

**Flags:**

- `--config` — config file path (default: /etc/jabali/config.toml)
- `--json` — output as JSON

### `jabali admin`

Operator-only administrative subcommands

```
jabali admin
```

#### `jabali admin backfill-usernames`

Assign a unique username to every user missing one (M54 Wave A)

```
jabali admin backfill-usernames [flags]
```

**Flags:**

- `--apply` — Write the derived usernames to the DB

#### `jabali admin purge-orphan-identities`

Delete Kratos identities with no matching panel user (frees stuck usernames)

```
jabali admin purge-orphan-identities [flags]
```

**Flags:**

- `--apply` — delete the orphan identities (default: dry-run)
- `--username` — only purge the orphan whose username or email matches this value

#### `jabali admin rebuild-kratos`

Recreate Kratos identities from panel users (DB-loss recovery, ADR-0034)

```
jabali admin rebuild-kratos [flags]
```

**Flags:**

- `--dry-run` — List target users and exit without writing
- `--expires-in` — Kratos recovery-code TTL (e.g. 1h, 24h) — operators typically need ≥24h to distribute (default `24h`)
- `--output` — CSV file to emit (email, new kratos_identity_id, recovery_link, status) (default `/tmp/jabali-recovery-tokens.csv`)
- `--yes` — Skip interactive confirmation prompt

#### `jabali admin relabel-identifiers`

Re-key Kratos login identifiers email -> username (M54 Wave C)

```
jabali admin relabel-identifiers [flags]
```

**Flags:**

- `--apply` — PATCH the identities (default: dry-run)

#### `jabali admin slice-cutover`

Migrate FPM from global master to per-user masters and mask distro units

```
jabali admin slice-cutover [flags]
```

**Flags:**

- `--dry-run` — run preflight only; do not mask global FPM or probe

### `jabali aide`

AIDE file integrity monitor (M42) operator commands

```
jabali aide
```

#### `jabali aide rebuild`

Re-baseline the AIDE database after a deliberate change

```
jabali aide rebuild [flags]
```

**Flags:**

- `--dry-run` — Preview plan without rebuilding (mutually exclusive with --full)
- `--full` — Confirm full DB re-init
- `--paths` — Partial re-baseline: only promote changes matching this regex (e.g. '^/usr/local/bin/jabali-'); refuses if changes outside the regex are detected

#### `jabali aide status`

Print AIDE DB age + last check summary

```
jabali aide status
```

### `jabali app`

Manage one-click app installs (direct DB — M20-safe)

```
jabali app
```

#### `jabali app delete`

Delete an installed app (direct DB + agent teardown — M20-safe)

```
jabali app delete <install-id|domain-name> [flags]
```

**Flags:**

- `--force` — Skip confirmation prompt

#### `jabali app e2e`

Install every app on a domain, report pass/fail, then delete

```
jabali app e2e [flags]
```

**Flags:**

- `--base-subdir` — Subdir prefix; each app installs under <prefix>_<app>_<rand> (default `e2e`)
- `--domain` — Domain name or ULID to install all apps under (required)
- `--keep` — Don't delete installs after the run (debug)
- `--only` — Only run these app_types (comma-separated) (default `[]`)
- `--skip` — Skip these app_types (comma-separated) (default `[]`)
- `--stop-on-fail` — Stop the sweep after the first failure
- `--wait-timeout` — Per-app install timeout in seconds (default `600`)

#### `jabali app get`

Show one installed app (direct DB — M20-safe)

```
jabali app get <install-id|domain-name>
```

#### `jabali app install`

Install an app on a domain (direct service — M20-safe)

```
jabali app install [flags]
```

**Flags:**

- `--app-type` — App descriptor name (see `jabali app registry`)
- `--directory` — Subdirectory under docroot (empty = site root)
- `--domain` — Target domain name or ULID (e.g. example.com or 01KPR…)
- `--param` — Per-app param: --param key=value (value is JSON; repeat for multiple) (default `[]`)
- `--use-www` — Reachable at www.<domain> too
- `--user` — Owner user (email, username, or ULID; default: domain owner)
- `--wait` — Poll until status is ready or failed
- `--wait-timeout` — Seconds to wait when --wait is set (default `600`)

#### `jabali app list`

List installed apps (direct DB — M20-safe)

```
jabali app list
```

#### `jabali app registry`

List available app types and their parameter schemas (direct read — no DB)

```
jabali app registry
```

### `jabali apparmor`

AppArmor profile management (M40) operator commands

```
jabali apparmor
```

#### `jabali apparmor flip-mature`

Flip mature complain-mode profiles to enforce

```
jabali apparmor flip-mature [flags]
```

**Flags:**

- `--dry-run` — show what would change without invoking aa-enforce
- `--profile` — Flip a single profile only

#### `jabali apparmor status`

List jabali AppArmor profiles and modes

```
jabali apparmor status
```

### `jabali appsec`

CrowdSec AppSec config operator subcommands

```
jabali appsec
```

#### `jabali appsec render-config`

Write /etc/crowdsec/appsec-configs/jabali-appsec.yaml from internal/appseccfg.Render

```
jabali appsec render-config [flags]
```

**Flags:**

- `--reconcile` — preserve operator jabali-mode/jabali-countries header from existing file (default true) (default `true`)

### `jabali audit`

Query, verify, and prune the unified audit log (M49 / ADR-0106)

```
jabali audit
```

#### `jabali audit prune`

Delete audit rows older than --days (retention; ADR-0106)

```
jabali audit prune [flags]
```

**Flags:**

- `--days` — retention window in days (delete older) (default `365`)

#### `jabali audit query`

List recent audit events (admin/forensics view)

```
jabali audit query [flags]
```

**Flags:**

- `--limit` — max rows (1-1000) (default `50`)
- `--q` — search (action/target/actor_kind/result)

#### `jabali audit verify`

Recompute the hash chain and report tamper-evidence integrity

```
jabali audit verify
```

### `jabali automation-token`

Manage Automation API tokens (headless provisioning)

```
jabali automation-token
```

#### `jabali automation-token list`

List automation tokens (never reveals secrets)

```
jabali automation-token list
```

#### `jabali automation-token mint`

Mint an automation token; reveals the secret once

```
jabali automation-token mint <name> [flags]
```

**Flags:**

- `--scope` — grant a scope (repeatable), e.g. --scope read:status (default `[]`)

#### `jabali automation-token revoke`

Revoke an automation token

```
jabali automation-token revoke <id-or-name>
```

### `jabali backup`

Backup & restore subcommands (M30 — restic-backed; ADR-0075 / 0080)

```
jabali backup
```

#### `jabali backup account-restore`

Restore one account's backup snapshot via the agent (bypass UI)

```
jabali backup account-restore [flags]
```

**Flags:**

- `--apply` — apply home+db onto live system (false = staging-only smoke test) (default `true`)
- `--destination` — destination name (e.g. 'test', 'b2-prod')
- `--force` — required — restore overwrites home tree + reloads databases
- `--interactive` — force interactive prompts even when flags are set
- `--snapshot` — manifest snapshot id (long form preferred)
- `--target-user-id` — disaster recovery: panel row gone; use this ULID directly (pair with --target-username)
- `--target-username` — disaster recovery: system account name to chown into (pair with --target-user-id)
- `--user` — username (e.g. shukivaknin) — looks up panel row
- `--user-id` — user ULID — looks up panel row (alternative to --user)

#### `jabali backup copy`

[RETIRED] superseded by per-destination model (ADR-0080)

```
jabali backup copy
```

#### `jabali backup destination`

Manage backup destinations (local, sftp, s3, b2, azure, gcs, rest)

```
jabali backup destination
```

##### `jabali backup destination create`

Create a backup destination

```
jabali backup destination create [flags]
```

**Flags:**

- `--disabled` — create in disabled state
- `--env` — credential env: KEY=VALUE (repeatable) (default `[]`)
- `--env-stdin` — read additional KEY=VALUE lines from stdin
- `--kind` — destination kind: local|sftp|s3|b2|azure|gcs|rest (required)
- `--name` — destination name (required, unique)
- `--url` — restic repo URL (required)

##### `jabali backup destination delete`

Delete a backup destination

```
jabali backup destination delete <id-or-name> [flags]
```

**Flags:**

- `--force` — skip confirmation

##### `jabali backup destination get`

Show a backup destination

```
jabali backup destination get <id-or-name>
```

##### `jabali backup destination list`

List backup destinations

```
jabali backup destination list
```

##### `jabali backup destination rotate-password`

Rotate a backup destination's restic password (revealed once)

```
jabali backup destination rotate-password <id-or-name>
```

##### `jabali backup destination test`

Test connectivity (auto-inits restic repo if missing)

```
jabali backup destination test <id-or-name>
```

##### `jabali backup destination update`

Update a backup destination

```
jabali backup destination update <id-or-name> [flags]
```

**Flags:**

- `--disable` — mark destination disabled
- `--enable` — mark destination enabled
- `--env` — rewrite credential env: KEY=VALUE (repeatable; s3/b2/azure/gcs/rest/sftp secrets) (default `[]`)
- `--env-stdin` — read additional KEY=VALUE credential lines from stdin
- `--name` — new name
- `--url` — new restic repo URL (validated against existing kind)

#### `jabali backup retention`

Manage restic retention (forget + prune per destination)

```
jabali backup retention
```

##### `jabali backup retention apply`

Run restic forget per (schedule, destination) + prune per destination

```
jabali backup retention apply [flags]
```

**Flags:**

- `--dry-run` — Pass restic --dry-run to forget+prune (lists what would be removed; no destructive ops)

#### `jabali backup schedule`

Manage backup schedules

```
jabali backup schedule
```

##### `jabali backup schedule create`

Create a backup schedule

```
jabali backup schedule create [flags]
```

**Flags:**

- `--cron` — 5-field cron expression (e.g. '0 3 * * *')
- `--destination` — destination id or name (repeatable) (default `[]`)
- `--disabled` — create in disabled state
- `--include-system` — for account_backup: also fire system_backup each tick
- `--keep-daily` — restic forget --keep-daily (default `0`)
- `--keep-monthly` — restic forget --keep-monthly (default `0`)
- `--keep-weekly` — restic forget --keep-weekly (default `0`)
- `--kind` — schedule kind: account_backup|system_backup (required)
- `--preset` — preset: daily|weekly|monthly (mutually exclusive with --cron)
- `--user` — user id|email|username for account_backup fan-out (repeatable; empty=all non-admins) (default `[]`)

##### `jabali backup schedule delete`

Delete a backup schedule

```
jabali backup schedule delete <id> [flags]
```

**Flags:**

- `--force` — skip confirmation

##### `jabali backup schedule get`

Show a backup schedule with destinations + users

```
jabali backup schedule get <id>
```

##### `jabali backup schedule list`

List backup schedules

```
jabali backup schedule list
```

##### `jabali backup schedule run-now`

Trigger a schedule by advancing next_run_at to now (scheduler picks up within ≤60s)

```
jabali backup schedule run-now <id>
```

##### `jabali backup schedule update`

Update a backup schedule

```
jabali backup schedule update <id> [flags]
```

**Flags:**

- `--clear-destinations` — remove all destinations
- `--clear-users` — remove all users (= fan-out to all)
- `--cron` — new cron expression
- `--destination` — replace destinations (repeatable) (default `[]`)
- `--disable` — mark schedule disabled
- `--enable` — mark schedule enabled
- `--include-system` — true|false (account_backup only)
- `--keep-daily` —  (default `0`)
- `--keep-monthly` —  (default `0`)
- `--keep-weekly` —  (default `0`)
- `--preset` — preset: daily|weekly|monthly
- `--user` — replace users (repeatable) (default `[]`)

#### `jabali backup scheduler`

Backup scheduler ops (manual tick / debug)

```
jabali backup scheduler
```

##### `jabali backup scheduler tick`

Run one enqueue + dispatch pass of the backup scheduler synchronously

```
jabali backup scheduler tick
```

### `jabali cron`

Manage user cron jobs (systemd-user timers)

```
jabali cron
```

#### `jabali cron add`

Add a cron job (5-field cron, allowlisted commands only)

```
jabali cron add [flags]
```

**Flags:**

- `--command` — command to run (required, allowlisted)
- `--disabled` — create disabled
- `--name` — job name (required)
- `--schedule` — 5-field cron expression e.g. '*/15 * * * *' (required)
- `--user` — user (id|email|username) (required)

#### `jabali cron delete`

Delete a cron job (reconciler removes the timer on next tick)

```
jabali cron delete <job-id> [flags]
```

**Flags:**

- `--force` — skip confirmation

#### `jabali cron http-trigger`

Fetch a self-domain URL with SSRF guard + rebind-safe IP pinning (internal cron exec helper)

```
jabali cron http-trigger <url>
```

#### `jabali cron list`

List cron jobs (filtered by user, or all)

```
jabali cron list [flags]
```

**Flags:**

- `--user` — filter by user (id|email|username); empty = all

#### `jabali cron run-now`

Run a cron job immediately via the agent (synchronous)

```
jabali cron run-now <job-id>
```

#### `jabali cron update`

Update a cron job

```
jabali cron update <job-id> [flags]
```

**Flags:**

- `--command` — 
- `--disable` — mark job disabled
- `--enable` — mark job enabled
- `--name` — 
- `--schedule` — 

### `jabali db`

Manage user databases (mariadb / postgres)

```
jabali db
```

#### `jabali db create`

Create a database for a user

```
jabali db create [flags]
```

**Flags:**

- `--as-admin` — Skip the username prefix (admin-only DB names)
- `--engine` — Engine: mariadb | postgres (default `mariadb`)
- `--name` — Database name (without user prefix) — required
- `--user` — User (email or username) — required

#### `jabali db delete`

Delete a database by ID

```
jabali db delete [flags]
```

**Flags:**

- `--id` — Database ID (ULID)

#### `jabali db list`

List databases (filtered by user, or all)

```
jabali db list [flags]
```

**Flags:**

- `--user` — Filter by user (email or username)

#### `jabali db user`

Manage database users (mariadb / postgres)

```
jabali db user
```

##### `jabali db user create`

Create a database user (auto-generates password if --password omitted)

```
jabali db user create [flags]
```

**Flags:**

- `--as-admin` — Skip the panel-username prefix (admin-only DB user names)
- `--engine` — Engine: mariadb | postgres (default `mariadb`)
- `--name` — DB user name (without panel-username prefix) — required
- `--password` — Password (auto-generated ULID if omitted; revealed once)
- `--user` — Panel user (email or username) — required

##### `jabali db user delete`

Delete a database user by ID

```
jabali db user delete [flags]
```

**Flags:**

- `--id` — DB user ID (ULID)

##### `jabali db user grant`

Grant a db user privileges on a database

```
jabali db user grant [flags]
```

**Flags:**

- `--db-name` — Database name (with panel-prefix) — required
- `--db-user-id` — DB user ID (ULID) — required
- `--level` — Shortcut: 'rw' or 'ro' (alternative to --privileges)
- `--privileges` — MariaDB privilege list (e.g. SELECT,INSERT,UPDATE) (default `[]`)

###### `jabali db user grant revoke`

Revoke a single grant, keeping the database user

```
jabali db user grant revoke <grant-id>
```

###### `jabali db user grant update`

Change a grant's level (rw|ro)

```
jabali db user grant update <grant-id> [flags]
```

**Flags:**

- `--level` — new grant level: 'rw' or 'ro' (required)

##### `jabali db user list`

List database users (filtered by panel user, or all)

```
jabali db user list [flags]
```

**Flags:**

- `--user` — Filter by panel user (email or username)

##### `jabali db user rotate-password`

Rotate a database user's password (revealed once)

```
jabali db user rotate-password <db-user-id>
```

### `jabali docker`

Manage the docker engine + app-marketplace host (M48/M49)

```
jabali docker
```

#### `jabali docker disable`

Disable the marketplace toggle (keeps docker installed)

```
jabali docker disable
```

#### `jabali docker enable`

Install docker engine + flip Server Settings toggle

```
jabali docker enable
```

#### `jabali docker enable-tenant`

Enable userns-remap + tenant docker apps on this host (M49, GH #170)

```
jabali docker enable-tenant [flags]
```

**Flags:**

- `--yes` — proceed without the interactive confirmation

#### `jabali docker status`

Show docker engine status (active, marketplace toggle state)

```
jabali docker status
```

### `jabali docker-app`

Manage M48 docker-app catalog installs (admin-only)

```
jabali docker-app
```

#### `jabali docker-app backups`

List restic backups taken for this install

```
jabali docker-app backups <id>
```

#### `jabali docker-app catalog`

List entries in the installed catalog

```
jabali docker-app catalog
```

#### `jabali docker-app delete`

Uninstall a docker app (stops the stack, removes its row)

```
jabali docker-app delete <id> [flags]
```

**Flags:**

- `--keep-volumes` — keep /var/lib/jabali/docker-apps/<slug> data on disk

#### `jabali docker-app install`

Install a catalog entry (creates the docker_apps row + dispatches the agent)

```
jabali docker-app install <slug> [flags]
```

**Flags:**

- `--cpu` — cgroup CPU limit (e.g. 1.0). Catalog default when omitted.
- `--env` — KEY=VALUE override (repeatable) (default `[]`)
- `--memory` — memory limit (e.g. 512m). Catalog default when omitted.
- `--name` — install name (lowercase, ^[a-z0-9-]{1,32}$)
- `--pids` — pids cgroup limit. Catalog default when omitted. (default `0`)
- `--update-mode` — manual|auto (default `manual`)

#### `jabali docker-app list`

List installed docker apps

```
jabali docker-app list
```

#### `jabali docker-app logs`

Tail container logs

```
jabali docker-app logs <id> [flags]
```

**Flags:**

- `--lines` — lines to tail (default `200`)
- `--service` — compose service name (default: first service)

#### `jabali docker-app rebuild`

Force-recreate (docker compose up --force-recreate)

```
jabali docker-app rebuild <id>
```

#### `jabali docker-app restart`

Restart an install

```
jabali docker-app restart <id>
```

#### `jabali docker-app start`

Start a stopped install

```
jabali docker-app start <id>
```

#### `jabali docker-app status`

Show full status of an installed app (DB row + agent status)

```
jabali docker-app status <id>
```

#### `jabali docker-app stop`

Stop a running install

```
jabali docker-app stop <id>
```

#### `jabali docker-app update`

Pull the latest image and re-create the stack

```
jabali docker-app update <id>
```

### `jabali domain`

Manage hosted domains

```
jabali domain
```

#### `jabali domain catchall`

Manage per-domain catch-all routing

```
jabali domain catchall
```

##### `jabali domain catchall clear`

Remove the catch-all rule for a domain

```
jabali domain catchall clear <domain-name-or-id>
```

##### `jabali domain catchall set`

Route mail to unknown@<domain> to --target

```
jabali domain catchall set <domain-name-or-id> [flags]
```

**Flags:**

- `--target` — Destination email address (required)

##### `jabali domain catchall show`

Print the current catch-all target

```
jabali domain catchall show <domain-name-or-id>
```

#### `jabali domain create`

Create a new domain (direct DB; bypasses HTTP auth — M20-safe)

```
jabali domain create [flags]
```

**Flags:**

- `--doc-root` — Document root (optional, auto-generated if not provided)
- `--name` — Domain name (required)
- `--user` — User email, username, or ULID (required)

#### `jabali domain delete`

Delete a domain (direct DB; reconciler tears down nginx — M20-safe)

```
jabali domain delete <domain-name|domain-id> [flags]
```

**Flags:**

- `--force` — Skip confirmation prompt

#### `jabali domain disable`

Disable a domain (direct DB — M20-safe)

```
jabali domain disable <domain-name|domain-id>
```

#### `jabali domain disclaimer`

Manage per-domain outbound disclaimer

```
jabali domain disclaimer
```

##### `jabali domain disclaimer clear`

Disable + remove the outbound disclaimer for a domain

```
jabali domain disclaimer clear <domain-name-or-id>
```

##### `jabali domain disclaimer set`

Set + enable the outbound disclaimer for a domain

```
jabali domain disclaimer set <domain-name-or-id> [flags]
```

**Flags:**

- `--file` — Read disclaimer from this file (overrides --text)
- `--text` — Disclaimer text (UTF-8, plain or HTML)

##### `jabali domain disclaimer show`

Print the current disclaimer for a domain

```
jabali domain disclaimer show <domain-name-or-id>
```

#### `jabali domain email-disable`

Disable email for a domain (keeps DKIM key per ADR-0043)

```
jabali domain email-disable <domain-name-or-id>
```

#### `jabali domain email-dkim-rotate`

Rotate the domain's DKIM keypair (ADR-0043; operator-driven, not automatic)

```
jabali domain email-dkim-rotate <domain-name-or-id>
```

#### `jabali domain email-enable`

Enable email for a domain (generates DKIM + publishes DNS records)

```
jabali domain email-enable <domain-name-or-id>
```

#### `jabali domain enable`

Enable a domain (direct DB — M20-safe)

```
jabali domain enable <domain-name|domain-id>
```

#### `jabali domain fix-perms`

Repair docroot group/setgid (www-data) for all of a user's domains

```
jabali domain fix-perms <username>
```

#### `jabali domain list`

List domains (direct DB — M20-safe)

```
jabali domain list
```

#### `jabali domain prune-orphans`

List sites in nginx sites-enabled that have no panel DB row (and optionally delete them)

```
jabali domain prune-orphans [flags]
```

**Flags:**

- `--apply` — Actually delete orphans (default: dry-run)

### `jabali limits`

Per-user resource limits (cgroups v2 + POSIX quota + nginx)

```
jabali limits
```

#### `jabali limits apply`

Re-apply effective limits for one user

```
jabali limits apply <username>
```

#### `jabali limits check`

Probe host for M18 prerequisites (cgroups v2, /home fs, nginx modules)

```
jabali limits check
```

#### `jabali limits package`

Bulk limits operations across every user of a package

```
jabali limits package
```

##### `jabali limits package apply`

Re-apply limits to every user of the given package

```
jabali limits package apply <package_id> [flags]
```

**Flags:**

- `--dry-run` — print what would be applied without making agent calls

#### `jabali limits status`

Show live resource usage for one user

```
jabali limits status <username>
```

### `jabali mailbox`

Manage mailboxes (M6 Email via Stalwart)

```
jabali mailbox
```

#### `jabali mailbox autoresponder`

Manage per-mailbox vacation responders

```
jabali mailbox autoresponder
```

##### `jabali mailbox autoresponder clear`

Disable + delete the autoresponder for a mailbox

```
jabali mailbox autoresponder clear <email>
```

##### `jabali mailbox autoresponder set`

Enable an autoresponder for a mailbox

```
jabali mailbox autoresponder set <email> [flags]
```

**Flags:**

- `--body` — Plain text body (optional if --html-body set)
- `--from` — Start date (RFC3339, e.g. 2026-05-01T00:00:00Z)
- `--html-body` — HTML body (optional)
- `--subject` — Subject line (required)
- `--to` — End date (RFC3339)

##### `jabali mailbox autoresponder show`

Print the current autoresponder for a mailbox

```
jabali mailbox autoresponder show <email>
```

#### `jabali mailbox create`

Create a mailbox (password shown once if auto-generated)

```
jabali mailbox create [flags]
```

**Flags:**

- `--domain` — Domain name or ID (required)
- `--local` — Local part, e.g. "alice" (required)
- `--password` — Explicit password (omit to auto-generate)
- `--quota-mb` — Disk quota in MiB (default 1024) (default `0`)

#### `jabali mailbox delete`

Delete a mailbox (agent destroys Stalwart account first)

```
jabali mailbox delete <email> [flags]
```

**Flags:**

- `--force` — Skip confirmation prompt

#### `jabali mailbox forwarder`

Manage per-mailbox aliases + external forwards

```
jabali mailbox forwarder
```

##### `jabali mailbox forwarder add`

Add an alias or external forwarder to a mailbox

```
jabali mailbox forwarder add <email> [flags]
```

**Flags:**

- `--keep-copy` — type=external: keep a copy in the mailbox (Sieve redirect :copy)
- `--local` — Alias local part (required for type=alias)
- `--target` — External destination email (required for type=external)
- `--type` — alias | external (required)

##### `jabali mailbox forwarder list`

List forwarders attached to a mailbox

```
jabali mailbox forwarder list <email>
```

##### `jabali mailbox forwarder remove`

Delete a forwarder by ID (find via 'forwarder list')

```
jabali mailbox forwarder remove <forwarder-id>
```

#### `jabali mailbox list`

List mailboxes in a domain

```
jabali mailbox list [flags]
```

**Flags:**

- `--domain` — Domain name or ID (required)

#### `jabali mailbox passwd`

Rotate a mailbox password (auto-generated and shown once if --password omitted)

```
jabali mailbox passwd <email> [flags]
```

**Flags:**

- `--password` — Explicit new password (omit to auto-generate)

#### `jabali mailbox set-quota`

Update a mailbox disk quota (in MiB)

```
jabali mailbox set-quota <email> <mb>
```

#### `jabali mailbox shares`

Manage shared mailbox folders (M6.5)

```
jabali mailbox shares
```

##### `jabali mailbox shares add`

Grant a target mailbox shared access to the owner's mailbox

```
jabali mailbox shares add [flags]
```

**Flags:**

- `--owner` — Owner mailbox email (required)
- `--rights` — Preset: ro | rw | admin (default rw) (default `rw`)
- `--shared-with` — Mailbox to grant share to (required)

##### `jabali mailbox shares list`

List shares for a given owner email

```
jabali mailbox shares list [flags]
```

**Flags:**

- `--owner` — Owner mailbox email (required)

##### `jabali mailbox shares remove`

Revoke a share by ID

```
jabali mailbox shares remove [flags]
```

**Flags:**

- `--id` — Share ID (ULID, from `jabali mailbox shares list`)

### `jabali malware-purge`

Hard-delete terminated malware quarantine rows past retention (M33)

```
jabali malware-purge
```

### `jabali migrate`

Database migration commands

```
jabali migrate
```

#### `jabali migrate import`

Run (or resume) a migration job through the four-stage pipeline

```
jabali migrate import [flags]
```

**Flags:**

- `--job-id` — migration_jobs.id (ULID) — required
- `--keep-staging` — do NOT delete /var/lib/jabali-migrations/<job-id>/ after run (debug aid)
- `--target-email` — destination user email (only used when auto-creating)
- `--target-package-id` — hosting package ULID (only used when auto-creating)
- `--target-password` — destination user password (only used when auto-creating; ≥10 chars)
- `--target-user` — destination jabali username — auto-created if --target-email + --target-password supplied

#### `jabali migrate pull-source`

Connect to source via SSH, run kind-appropriate backup, pull + extract tarball

```
jabali migrate pull-source [flags]
```

**Flags:**

- `--job-id` — migration_jobs.id (ULID) — required
- `--ssh-user` — SSH login on the source (default 'root') (default `root`)

#### `jabali migrate reap-secrets`

Wipe per-job migration-secrets env files + stale tarball/extracted dirs

```
jabali migrate reap-secrets [flags]
```

**Flags:**

- `--dry-run` — List would-delete paths without removing them
- `--staging-max-age` — Reap /var/lib/jabali-migrations/<id>/ only when the job has been terminal at least this long (default 168h = 7d; pass 0 to wipe immediately) (default `168h0m0s`)

#### `jabali migrate restore`

One-shot offline restore from a cpmove tarball (create job + stage + import)

```
jabali migrate restore [flags]
```

**Flags:**

- `--cpanel` — source is a cPanel cpmove / WHM pkgacct tarball
- `--file` — path to the cpmove tarball (cpmove-<user>.tar.gz) — required
- `--keep-staging` — keep /var/lib/jabali-migrations/<job-id>/ after the run (debug)
- `--restore-file` — alias of --file
- `--source-host` — informational source host (offline restore leaves this empty)
- `--source-user` — cPanel account (default: derived from the cpmove filename)
- `--target-email` — destination email (only used when auto-creating the user)
- `--target-package-id` — hosting package ULID (auto-create only)
- `--target-password` — destination password (auto-create only; ≥10 chars)
- `--target-user` — destination jabali username (default: the source account)

#### `jabali migrate up`

Run pending database migrations

```
jabali migrate up
```

### `jabali nspawn`

Manage SSH sandbox nspawn images (M13)

```
jabali nspawn
```

#### `jabali nspawn build`

debootstrap a deterministic, immutable nspawn rootfs

```
jabali nspawn build [flags]
```

**Flags:**

- `--codename` — image family (e.g. debian-13) (default `debian-13`)
- `--includes` — comma-separated debootstrap --include list (default `bash,coreutils,procps,findutils,grep,sed,gawk,less,nano,ca-certificates,git,curl,wget,vim-tiny,php-cli,php-mysql,php-curl,php-xml,php-mbstring,php-zip,php-gd,unzip,rsync,mariadb-client`)
- `--snapshot` — snapshot.debian.org timestamp YYYYMMDDTHHMMSSZ (mandatory)
- `--suite` — debootstrap suite (trixie, bookworm, ...) (default `trixie`)
- `--version` — image version label (e.g. v1, v2)

#### `jabali nspawn list`

List sealed nspawn images

```
jabali nspawn list
```

#### `jabali nspawn prune`

Remove sealed images that no user is pinned to

```
jabali nspawn prune [flags]
```

**Flags:**

- `--dry-run` — explicit dry-run (default; mutually exclusive with --yes)
- `--yes` — actually delete (default: dry-run)

### `jabali package`

Manage hosting packages

```
jabali package
```

#### `jabali package create`

Create a hosting package (direct DB — M20-safe)

```
jabali package create [flags]
```

**Flags:**

- `--bw-mb` — bandwidth quota in MB (0=unlimited) (default `0`)
- `--cgi` — enable CGI
- `--cpu` — CPU quota percent across all cores (0=unlimited) (default `0`)
- `--databases` — max databases (0=unlimited) (default `0`)
- `--disk-mb` — disk quota in MB (0=unlimited) (default `0`)
- `--domains` — max domains (0=unlimited) (default `0`)
- `--emails` — max email accounts (0=unlimited) (default `0`)
- `--io-read-mbps` — disk read bandwidth limit in MB/s (0=unlimited) (default `0`)
- `--io-write-mbps` — disk write bandwidth limit in MB/s (0=unlimited) (default `0`)
- `--max-tasks` — max processes/threads per user slice (0=unlimited) (default `0`)
- `--memory-mb` — memory limit in MB (0=unlimited) (default `0`)
- `--name` — package name (required)
- `--ssh` — enable SSH access

#### `jabali package delete`

Delete a hosting package (direct DB — M20-safe)

```
jabali package delete <package-id> [flags]
```

**Flags:**

- `--force` — skip confirmation

#### `jabali package edit`

Edit a hosting package (direct DB — M20-safe)

```
jabali package edit <package-id> [flags]
```

**Flags:**

- `--bw-mb` — bandwidth MB (default `0`)
- `--cgi` — CGI access (true/false)
- `--cpu` — CPU quota percent (default `0`)
- `--databases` — max databases (default `0`)
- `--disk-mb` — disk quota MB (default `0`)
- `--domains` — max domains (default `0`)
- `--emails` — max emails (default `0`)
- `--io-read-mbps` — io read MB/s (default `0`)
- `--io-write-mbps` — io write MB/s (default `0`)
- `--max-tasks` — max processes/threads (default `0`)
- `--memory-mb` — memory limit MB (default `0`)
- `--name` — package name
- `--ssh` — SSH access (true/false)

#### `jabali package list`

List hosting packages (direct DB — M20-safe)

```
jabali package list
```

### `jabali panel-primary`

Manage the panel's primary mail domain row (ADR-0048)

```
jabali panel-primary
```

#### `jabali panel-primary ensure`

Ensure a panel-primary domain row exists for the given hostname

```
jabali panel-primary ensure [flags]
```

**Flags:**

- `--hostname` — panel hostname (e.g. jabali-panel.local)

### `jabali pdns`

PowerDNS helpers (recursor forwarders, etc.)

```
jabali pdns
```

#### `jabali pdns backfill`

Converge /etc/powerdns/recursor.forwards with the panel database

```
jabali pdns backfill [flags]
```

**Flags:**

- `--dry-run` — explicit dry-run (default; mutually exclusive with --yes)
- `--no-confirm` — skip the y/N confirmation when --yes is used (for scripted runs; otherwise set JABALI_PDNS_BACKFILL_NONINTERACTIVE=1)
- `--verbose` — print per-zone detail
- `--yes` — apply the plan (default is dry-run)

#### `jabali pdns dnssec`

Per-domain DNSSEC management (ADR-0057)

```
jabali pdns dnssec
```

##### `jabali pdns dnssec disable`

Disable DNSSEC (removes keys + rectifies)

```
jabali pdns dnssec disable <domain> [flags]
```

**Flags:**

- `--force` — skip confirmation

##### `jabali pdns dnssec ds`

Print DS records to publish at the parent registrar

```
jabali pdns dnssec ds <domain>
```

##### `jabali pdns dnssec enable`

Enable DNSSEC for a zone (creates KSK+ZSK, rectifies, persists keys)

```
jabali pdns dnssec enable <domain>
```

##### `jabali pdns dnssec status`

Show cached DNSSEC keys for a domain

```
jabali pdns dnssec status <domain>
```

### `jabali per-user-egress`

Per-user PHP-FPM egress firewall (M34) operator commands

```
jabali per-user-egress
```

#### `jabali per-user-egress flip-mature`

Flip mature LEARNING policies to ENFORCED

```
jabali per-user-egress flip-mature [flags]
```

**Flags:**

- `--dry-run` — show what would change without writing to DB
- `--soak-days` — minimum LEARNING age before auto-flip to ENFORCED (default `7`)

### `jabali php`

PHP version + extension + per-user pool management

```
jabali php
```

#### `jabali php ext`

Manage PHP extensions (server-wide per PHP version)

```
jabali php ext
```

##### `jabali php ext disable`

Disable an installed extension via phpdismod

```
jabali php ext disable <ext> [flags]
```

**Flags:**

- `--version` — PHP version (required)

##### `jabali php ext enable`

Enable an installed extension via phpenmod

```
jabali php ext enable <ext> [flags]
```

**Flags:**

- `--version` — PHP version (required)

##### `jabali php ext install`

Install (apt) an extension package

```
jabali php ext install <ext> [flags]
```

**Flags:**

- `--version` — PHP version (required)

##### `jabali php ext list`

List PHP extensions and their installed/enabled state for a version

```
jabali php ext list [flags]
```

**Flags:**

- `--version` — PHP version (e.g. 8.4) (required)

##### `jabali php ext remove`

Remove (apt) an extension package

```
jabali php ext remove <ext> [flags]
```

**Flags:**

- `--version` — PHP version (required)

#### `jabali php pool`

Per-user PHP-FPM pool

```
jabali php pool
```

##### `jabali php pool get`

Show a user's PHP pool

```
jabali php pool get <user>
```

##### `jabali php pool reapply-all`

Mark all active pools pending so the reconciler re-renders them from the current template

```
jabali php pool reapply-all
```

##### `jabali php pool set`

Set a user's PHP version (reconciler regenerates pool conf)

```
jabali php pool set [flags]
```

**Flags:**

- `--user` — user (id|email|username) (required)
- `--version` — PHP version e.g. 8.4 (required)

#### `jabali php version`

Manage installed PHP versions

```
jabali php version
```

##### `jabali php version install`

Install a PHP version (e.g. 8.4) — installs base + required extensions, starts php<v>-fpm

```
jabali php version install <version>
```

##### `jabali php version list`

List installed PHP versions

```
jabali php version list
```

##### `jabali php version reload`

Reload php<v>-fpm.service (zero-downtime SIGUSR2)

```
jabali php version reload <version>
```

### `jabali python-app`

Manage Python apps (ADR-0131; admin-only)

```
jabali python-app
```

#### `jabali python-app delete`

Stop + remove an app (app files are kept)

```
jabali python-app delete <app-id>
```

#### `jabali python-app list`

List Python apps

```
jabali python-app list
```

#### `jabali python-app logs`

Show an app's recent logs

```
jabali python-app logs <app-id> [flags]
```

**Flags:**

- `--lines` — number of log lines (default `200`)

#### `jabali python-app restart`

Restart an app

```
jabali python-app restart <app-id>
```

#### `jabali python-app start`

Start an app

```
jabali python-app start <app-id>
```

#### `jabali python-app stop`

Stop an app

```
jabali python-app stop <app-id>
```

### `jabali release`

Release-channel management (stable/development)

```
jabali release
```

#### `jabali release promote`

Promote a reviewed build to the stable channel (move the `stable` tag)

```
jabali release promote [<commit-ish>]
```

### `jabali repair`

Detect and fix known deployment-host issues

```
jabali repair [flags]
```

**Flags:**

- `--all` — Fix every issue including destructive ones
- `--apparmor-profiles-disabled` — Fix only: jabali AppArmor profiles exist but are disabled
- `--apparmor-profiles-missing` — Fix only: jabali AppArmor profiles absent from /etc/apparmor.d/
- `--auto` — Fix every non-destructive (safe) issue
- `--bulwark-jwt-secret` — Fix only: Bulwark webmail-SSO secret poisoned / out of sync with bulwark.env (mail impersonation 'Invalid signature')
- `--crowdsec-bouncer-key` — Fix only: crowdsec-firewall-bouncer crash-loops with stale LAPI key
- `--daemon-reload` — Fix only: systemd has unloaded unit-file changes on disk
- `--diagnose` — Report broken conditions without fixing
- `--docroot-www-data-group` — Fix only: web docroot files not group www-data / dirs not setgid (nginx 403 on newly uploaded media)
- `--etc-jabali-perms` — Fix only: /etc/jabali not traversable by hosting users (SSH/SFTP locked out — sandbox-mode unreadable)
- `--git-ownership` — Fix only: /opt/jabali-panel/.git owned by wrong user
- `--git-pointer` — Fix only: /opt/jabali-panel/.git is a corrupted worktree pointer
- `--git-stale-worktrees` — Fix only: /opt/jabali-panel/.git/worktrees has stale entries
- `--nginx-config-invalid` — Fix only: jabali-default/jabali-panel.conf has `http2 on;` on nginx<1.25.1 (nginx -t fails, reloads rejected)
- `--nginx-missing-includes` — Fix only: panel :8443 vhost includes a missing optional snippet (phpMyAdmin/Adminer) — nginx -t fails, nothing on 8443 (GH #217)
- `--node-modules` — Fix only: panel-ui/node_modules partial (missing .bin/tsc)
- `--ondrej-nginx-ppa` — Fix only: stale ondrej/nginx PPA in apt sources (404 on noble)
- `--orphan-migration-staging` — Fix only: /var/lib/jabali-migrations/* dirs for jobs already terminal in DB
- `--orphan-slices` — Fix only: jabali-user-*.slice units exist for deleted unix users
- `--uploads-dir` — Fix only: /var/lib/jabali-uploads missing or wrong perms
- `--yes` — Skip interactive confirmation for destructive repairs

### `jabali serve`

Start the Jabali Panel HTTP(S) server

```
jabali serve
```

### `jabali shared-resource`

Manage shared mail resources — calendars, contacts, files (M52)

```
jabali shared-resource
```

#### `jabali shared-resource create`

Create a shared resource

```
jabali shared-resource create [flags]
```

**Flags:**

- `--display-name` — display name
- `--domain` — domain ID (ULID)
- `--kind` — mailbox|calendar|addressbook|files
- `--name` — host address local part

#### `jabali shared-resource grant`

Add or update a grant (upsert into the grant set)

```
jabali shared-resource grant [flags]
```

**Flags:**

- `--grantee` — grantee mailbox/group ID
- `--grantee-kind` — mailbox|group (default `mailbox`)
- `--resource` — shared resource ID
- `--rights` — read|readwrite|admin (default `read`)

#### `jabali shared-resource grants`

List grants on a shared resource

```
jabali shared-resource grants [flags]
```

**Flags:**

- `--resource` — shared resource ID

#### `jabali shared-resource list`

List shared resources in a domain

```
jabali shared-resource list [flags]
```

**Flags:**

- `--domain` — domain ID (ULID)

#### `jabali shared-resource remove`

Delete a shared resource (reconciler tears down the host principal)

```
jabali shared-resource remove [flags]
```

**Flags:**

- `--resource` — shared resource ID

#### `jabali shared-resource revoke`

Remove a grantee's grant from a shared resource

```
jabali shared-resource revoke [flags]
```

**Flags:**

- `--grantee` — grantee ID to revoke
- `--resource` — shared resource ID

### `jabali ssh-key`

Manage user SSH authorized keys

```
jabali ssh-key
```

#### `jabali ssh-key add`

Add an SSH public key for a user

```
jabali ssh-key add [flags]
```

**Flags:**

- `--name` — key label (required)
- `--pub-key` — raw public key (e.g. 'ssh-ed25519 AAAA... user@host')
- `--pub-key-file` — path to public key file
- `--pub-key-stdin` — read public key from stdin
- `--user` — user (id|email|username) (required)

#### `jabali ssh-key delete`

Delete an SSH key

```
jabali ssh-key delete <key-id> [flags]
```

**Flags:**

- `--force` — skip confirmation

#### `jabali ssh-key list`

List SSH keys (filtered by --user, or all without filter)

```
jabali ssh-key list [flags]
```

**Flags:**

- `--all` — list every user's keys (default when --user is omitted)
- `--user` — filter by user (id|email|username); empty = all

### `jabali ssl`

Manage Let's Encrypt SSL certificates

```
jabali ssl
```

#### `jabali ssl disable`

Disable SSL for a domain (reconciler will revoke cert)

```
jabali ssl disable <domain>
```

#### `jabali ssl enable`

Enable SSL for a domain (reconciler will issue cert within ≤60s)

```
jabali ssl enable <domain>
```

#### `jabali ssl list`

List SSL certificates (optionally filtered by user)

```
jabali ssl list [flags]
```

**Flags:**

- `--user` — filter by user (id|email|username)

#### `jabali ssl renew`

Renew SSL cert via certbot (synchronous, calls agent)

```
jabali ssl renew <domain> [flags]
```

**Flags:**

- `--force` — force renewal even if cert is not due

### `jabali sso`

SSO (Single Sign-On) management commands

```
jabali sso
```

#### `jabali sso prune-tokens`

Manually purge expired SSO tokens

```
jabali sso prune-tokens
```

#### `jabali sso rotate-key`

Rotate the SSO encryption key

```
jabali sso rotate-key [flags]
```

**Flags:**

- `--current-key` — path to current encryption key (default `/etc/jabali/sso_key.txt`)
- `--new-key` — path to new encryption key (required)

### `jabali sso-reap`

Sweep stranded jabali-sso-<nonce>.php files (M22 reaper)

```
jabali sso-reap
```

### `jabali system`

System information and services

```
jabali system
```

#### `jabali system info`

Show system info (hostname, uptime, CPU, memory, disk)

```
jabali system info
```

#### `jabali system restore`

Restore a system backup onto this host (CLI; ADR-0080)

```
jabali system restore [flags]
```

**Flags:**

- `--apply` — after staging, apply selected stages onto live host (default true) (default `true`)
- `--apply-stage` — restrict apply to named stages (repeatable). Empty = panel_db + panel_config + tls (the safe defaults) (default `[]`)
- `--credentials-ref` — absolute path to env file with backend creds (root:root 0600)
- `--extra-option` — restic -o KEY=VALUE flag body (repeatable) (default `[]`)
- `--force` — required — restore overwrites the running panel
- `--include-accounts` — also restore each linked account
- `--interactive` — force interactive prompts even when --remote-url is set
- `--password` — restic password (literal; overrides --password-file). Avoid in shell history; prefer --interactive
- `--password-file` — restic password file (default: /etc/jabali-panel/restic-repo.password)
- `--remote-url` — restic repo URL or local path (e.g. sftp:user@host:/path)
- `--snapshot` — system_manifest snapshot ID, or 'latest' to auto-pick newest

#### `jabali system services`

Show systemd service status

```
jabali system services
```

### `jabali ufw`

UFW utilities (M43 — port baseline only; IP decisions live in CrowdSec)

```
jabali ufw
```

#### `jabali ufw migrate-ip-bans`

Migrate UFW `from <IP>` deny rules to CrowdSec decisions (M43 Step 4)

```
jabali ufw migrate-ip-bans [flags]
```

**Flags:**

- `--dry-run` — Show what would migrate; make no changes
- `--no-cdn` — Confirm panel is not behind a CDN; bypasses the trusted_ips hard guard
- `--revert` — Restore UFW rules from snapshot and remove matching CrowdSec decisions
- `--yes` — Required for any destructive operation (migrate or revert)

### `jabali update`

Pull latest code, rebuild, migrate, and restart services

```
jabali update [flags]
```

**Flags:**

- `--force` — Run the full rebuild/restart cycle even when git pull found no new commits
- `--from-source` — Build binaries on this host instead of downloading the release tarball from Gitea Releases. Default is to download the tarball (90s update vs 5-10min source build). Use --from-source when offline, on a private fork, or to test uncommitted changes.

### `jabali user`

Manage panel users

```
jabali user
```

#### `jabali user 2fa-reset`

Strip TOTP + recovery codes from a user (CLI escape hatch when locked out)

```
jabali user 2fa-reset <email|username|user-id>
```

#### `jabali user create`

Create a new user (direct DB + Kratos; bypasses HTTP auth — M20-safe)

```
jabali user create [username] [flags]
```

**Flags:**

- `--admin` — grant admin role
- `--email` — user email (optional; a placeholder is synthesized from the username if omitted)
- `--name-first` — first name
- `--name-last` — last name
- `--password` — user password (required, min 10 chars)
- `--password-stdin` — read password from stdin (no prompt, no echo)
- `--username` — login username — the identifier users sign in with (required unless --email is given to derive one)

#### `jabali user delete`

Delete a user — destructive: domains, databases, mailboxes, OS account, /home, all related rows.

```
jabali user delete <email|username|user-id> [flags]
```

**Flags:**

- `--force` — skip confirmation prompt

#### `jabali user list`

List all users (direct DB — M20-safe)

```
jabali user list
```

#### `jabali user password`

Reset a user's password (auto-generates one if --password is omitted)

```
jabali user password <email|username|user-id> [flags]
```

**Flags:**

- `--expires-in` — TTL for recovery link (only with --link) (default `24h`)
- `--link` — emit a one-click recovery URL instead of setting the password directly
- `--password` — explicit new password (omit to auto-generate)
- `--password-stdin` — read new password from stdin (no prompt, no echo)

### `jabali version`

Print Jabali version, commit, and runtime info

```
jabali version
```

