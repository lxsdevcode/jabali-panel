# Blueprint — Per-user CLI PHP version (GH #184)

**Status:** SHIPPED 2026-06-14 (ADR-0126; live-verified on mx). Option B chosen. **Owner:** shuki + Claude. **Date:** 2026-06-14.

## Problem (GH #184)

The per-user pinned PHP version + its extensions apply to **FPM/web** (and to
app installers via `phpCLIFor()`), but **not** to the user's **interactive CLI,
Composer, or cron**. Those resolve `php` to `/usr/bin/php` = the
update-alternatives **system default** version, so:

- A user pinned to a non-default version gets the *default* at the shell.
- Extensions enabled for the pinned version look "missing" in `php -m`/Composer
  (they exist — but on the pinned version's CLI SAPI, not the default's).

Single root cause: **no per-user CLI `php` on the interactive/cron PATH.**
`phpenmod -v <v>` (no `-s`) already enables both cli+fpm SAPIs, so once the CLI
points at the right version, extensions come for free. Not reproducible on a
single-version host (10.0.3.14 has only 8.4) — needs ≥2 versions to surface.

## Decision

Give each user a CLI `php` pinned to their version, shadowing `/usr/bin/php`
on the interactive shell PATH and in cron. Composer and wp-cli use
`#!/usr/bin/env php`, so a PATH-shadowing `php` fixes them with no extra work.

The pinned binary is a **baked symlink** (`php → /usr/bin/php<v>`), written by
the agent when the version is set — NOT a runtime read of the pin file (that
file isn't bound inside the SSH sandbox).

**Open design choice for sign-off — wrapper location (Step 1):**
- **(A) Root-owned `/etc/jabali-panel/user-bin/<user>/`**, ro-bound into the SSH
  sandbox at a fixed mountpoint, prepended to PATH. Most tamper-proof; needs a
  sandbox bind + careful bind-order vs the `/run` tmpfs.
- **(B) `/home/<user>/.jabali/bin/`** (home is already rw-bound), the `.jabali`
  dir **root-owned 0755** so the tenant can't replace `php`. Simpler (no new
  bind); tenant owns the parent home but not the root-owned subdir.

Recommend **(B)** for simplicity unless the sandbox-bind in (A) is preferred for
defense-in-depth. Either way the symlink target is validated `php<v>` only.

## Steps

1. **Wrapper provisioning (agent).** New helper writes/refreshes the per-user
   bin dir: `php` (+ `phar`) → `/usr/bin/php<pinnedVersion>`. Target validated
   against `phpVersionPattern` + must exist. Hook into `php.pool.apply`
   (version set/change) and remove in `php.pool.remove`. Idempotent
   (no-change → no rewrite). Reuses the existing pin file as source of truth.

2. **Interactive SSH PATH (jabali-ssh-shell).** Prepend the per-user bin dir to
   the sandbox PATH (front, so it shadows `/usr/bin/php`); for option (A) also
   add the `--ro-bind`. Verify resolution order inside bwrap.

3. **Cron (cron_apply.go).** Make scheduled `php`/`wp` run the pinned version —
   set the generated unit's `Environment=PATH=<user-bin>:/usr/bin:/bin` (so a
   bare `php`/`wp` token resolves to the wrapper) rather than rewriting tokens.
   Keep the cronvalidate allowlist unchanged.

4. **install.sh.** Create the wrapper root dir (option A) or ensure the home
   `.jabali` convention is documented; per "install.sh is truth", required perms
   live here, not a cutover CLI.

5. **Reconciler convergence + lifecycle.** Ensure version-change and user-delete
   paths refresh/remove wrappers (gate the symlink rewrite behind a no-change
   compare to avoid churn). Backfill existing users on first apply.

6. **Docs + ADR.** ADR amending ADR-0023/0025 scope ("CLI follows the FPM
   version pin"); update `docs/site/user/php-settings.md` (CLI now matches your
   version) + a note that Composer works via the PATH `php`.

7. **Tests + live-verify.** Unit: wrapper path build + symlink-target
   validation + cron PATH line + no-change idempotency. **Live: install a 2nd
   PHP version on 10.0.3.14, pin a user to the non-default version, confirm
   interactive `php -v`, a cron `php -v`, and `composer`/`php -m` all use the
   pinned version + see its extensions.** (This is the real gate — the bug is
   invisible on a single-version host.)

## Risks / notes

- **Security:** symlink target is always a validated `/usr/bin/php<v>`, never a
  tenant-controlled path; wrapper dir not tenant-writable. Do not weaken the
  sandbox hardening to add the bind (CONVENTIONS "security over functionality").
- **Blast radius:** `php.pool.apply`, `jabali-ssh-shell`, `cron_apply.go`,
  install.sh. Shared `phpCLIFor` already exists — extend, don't duplicate.
- **Composer not bundled:** if the operator hasn't installed composer, that's
  separate; this only guarantees `php` (and thus `composer.phar`) version.
