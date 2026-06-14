# ADR-0126: Per-user CLI PHP version follows the FPM version pin

**Date**: 2026-06-14
**Status**: Accepted
**Deciders**: shuki + Claude
**Related**: ADR-0023 (M9 PHP-FPM pool manager), ADR-0025 (per-user systemd slices), ADR-0031 (PHP extensions), ADR-0067 (M13 SSH sandbox). Fixes GH #184.

## Context

The per-user pinned PHP version (and its extensions) was applied to the
**FPM/web** path (`jabali-fpm@<user>`, version-pinned) and to **app
installers** (via the existing `phpCLIFor()` helper), but **not** to the
user's **interactive shell, Composer, wp-cli, or cron**. Those resolve a
bare `php` to `/usr/bin/php` — the update-alternatives **system default**
version — so a user pinned to a non-default version got the wrong version
at the CLI. Because extensions are installed per PHP version, anything
enabled for the pinned version then looked "missing" when run at the
default version (GH #184: two symptoms, one root cause).

The panel's PHP management was deliberately FPM-centric (docs:
`php-settings.md` "Applied to every domain you own that runs PHP through
your per-user FPM pool"); per-user CLI was never wired.

## Decision

Each user gets a CLI `php` pinned to their version, shadowing
`/usr/bin/php` on the interactive shell + cron PATH. Composer and wp-cli
use `#!/usr/bin/env php`, so a PATH-shadowing `php` fixes them with no
extra handling.

- **Wrapper** (option B): `/home/<user>/.jabali/bin/php` → symlink to
  `/usr/bin/php<pinned>`. `.jabali` + `.jabali/bin` are **root-owned
  0755** so a tenant can't swap the wrapper. Best-effort, not a security
  boundary — a downgrade only affects the tenant's own CLI (no
  cross-tenant / privilege impact). The symlink target is always a
  validated `/usr/bin/php<version>` that exists; never tenant-controlled.
  Written by `ensureUserCLIPHP` (atomic, idempotent — a correct symlink
  is a no-op).
- **Provisioning**: `php.pool.apply` writes/refreshes the wrapper on
  version set/change (best-effort, never fails the FPM apply);
  `php.pool.remove` removes it. The reconciler only fires `php.pool.apply`
  for pending/error pools, so existing (active) users are backfilled by
  `BackfillUserCLIPHP` at agent startup (every `jabali update` restart
  converges them).
- **Interactive SSH**: `jabali-ssh-shell` prepends
  `<home>/.jabali/bin` to the sandbox PATH.
- **Cron**: generated units set
  `Environment=PATH=/home/<user>/.jabali/bin:…` so scheduled `php`/`wp`
  use the pinned version.

Wrapper location B (home dir) was chosen over A (root-owned
`/etc/jabali-panel/user-bin/<user>` ro-bound into the sandbox) for
simplicity — no extra bwrap bind. On a jabali host `/home/<user>` is
already root-owned (M18), so B is as tamper-resistant as A in practice.

## Alternatives Considered

- **`update-alternatives` per login / profile.d alias** — global, can't
  be per-user cleanly; an alias doesn't follow into Composer subprocesses
  (only a PATH binary does). Rejected.
- **Wrapper reads the pin file at runtime** — the pin
  (`/etc/jabali-panel/user-phpver/<user>`) isn't bound inside the SSH
  sandbox; a baked symlink resolves there (only `/usr` + `/home/<user>`
  are bound). Rejected.
- **Root-owned `/etc/jabali-panel/user-bin` ro-bound (option A)** — more
  defense-in-depth but needs a careful bwrap bind vs the `/run` tmpfs.
  Deferred; B is sufficient given root-owned homes.

## Consequences

### Positive
- CLI, Composer, wp-cli, and cron now match the panel-displayed PHP
  version + its extensions. Closes GH #184.
- Reconciler-free for active pools (startup backfill); idempotent, no
  reload churn (symlink no-op when already correct).
- Reuses `phpCLIFor` lineage; small surface.

### Security hardening (symlink TOCTOU)

The agent runs as root and writes under `/home/<user>`. Every path component
(`/home/<user>`, `.jabali`, `.jabali/bin`, the `php` entry) is `Lstat`'d and
refused if it is a symlink or not root-owned; chowns use `Lchown`; the wrapper
is replaced only when the existing entry is absent or already a symlink (never
over a regular file). So even if a future host had a tenant-writable home, a
planted symlink could not redirect root's mkdir/chown/symlink/rename into
privileged space — the agent refuses and logs instead. Live-verified: a
tenant-owned home is refused; a root-owned (jabali 0751) home is provisioned.

### Negative
- A tenant who renames their root-owned `.jabali` could shadow the
  wrapper — self-only downgrade, accepted (documented).
- The wrapper covers `php`; an operator who hasn't installed Composer is
  separate (this only guarantees the `php` version Composer runs under).

## Implementation

- `panel-agent/internal/commands/php_cli_wrapper.go` —
  `ensureUserCLIPHP` / `removeUserCLIPHP` / `replaceCLISymlink` /
  `BackfillUserCLIPHP`.
- Hooks: `php_pool_apply.go` (write), `php_pool_remove.go` (remove),
  `cmd/jabali-agent/main.go` (startup backfill).
- PATH: `cmd/jabali-ssh-shell/main.go` (interactive), `cron_apply.go`
  (cron unit `Environment=PATH`).
- Unit tests: `php_cli_wrapper_test.go` + cron-PATH assertion.
- **Live-verified on mx.jabali-panel.com** (PHP 8.3/8.4/8.5, default
  8.4): a user pinned to 8.5 got `/home/<u>/.jabali/bin/php` → php8.5;
  wrapper `php -v` reported 8.5.6 vs the 8.4 default; `.jabali` root-owned
  0755; startup backfill created it. (bwrap-via-`su` can't run the
  sandbox outside sshd, so the in-sandbox PATH is covered by unit test +
  the deterministic PATH string, not a live SSH login.)
- Plan: `plans/m-per-user-cli-php.md`.
