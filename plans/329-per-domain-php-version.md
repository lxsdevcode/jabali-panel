# Per-domain PHP version (GH #329) — implementation blueprint

**Branch:** `feat-329-per-domain-php-version`
**Status:** in progress
**Requester:** @lxsdevcode (GH #329)

## Goal

Let a domain run its own PHP **version** independently of sibling domains under
the same panel user, without forcing one-user-per-site.

## What already ships (do NOT rebuild)

- Per-domain PHP **settings** — `memory_limit`, `upload_max_filesize`,
  `post_max_size`, `max_input_vars`, `max_execution_time` are per-domain INI
  overrides rendered as `fastcgi_param PHP_VALUE` in each domain's vhost
  (`domain_create.go` `buildPHPValueParam`).
- Per-domain PHP on/off (`domain.php_pool_id` NULL = static).
- **Schema + repo foundation (M35.8 P6):** migration `000129` dropped
  `uniq_user_pool`, added `uniq_user_phpver (user_id, php_version)`;
  `PHPPoolRepository.FindByUserAndVersion` exists. Multiple pool ROWS per user
  are already allowed. **The runtime never caught up** — see gap below.

## The gap (what to build)

The one-pool-per-user model is still enforced at runtime by four layers:

1. **Agent** `php_pool_apply.go`: socket is `/run/php/jabali-<user>/fpm.sock`
   ("one socket per user, not per pool"); `globDeletePoolFiles(keep=[ver])`
   **deletes every other version's pool file** on each apply.
2. **systemd**: one master `jabali-fpm@<user>.service`; its per-user FPM config
   `include=`s exactly one pool file.
3. **Reconciler** `reconcilePHPPools`: `FindByUserID` (one pool), only ensures
   the user's default pool.
4. **Vhost**: `fastcgi_pass` → the single per-user socket.

**Latent bug this also fixes:** cpanel mixed-version restore inserts multiple
pool rows (that's *why* 000129 was added), but the reconciler applies only one
and the agent deletes the siblings — so a restored mixed-version account
silently collapses to one version. Fixed by Wave B.

## Design — ADDITIVE (key decision)

Keep the **default-version** pool/unit/socket **byte-identical** to today.
Only **non-default** versions are additive. Existing hosts see zero change
until an admin selects a non-default version for a domain.

- Pool identity = `(user_id, php_version)`.
- **Panel passes a `slug` to the agent** (new `php.pool.apply` param):
  - default-version pool → `slug = <user>`  (unchanged path scheme)
  - non-default pool → `slug = <user>-php<ver>` (e.g. `alice-php8.2`)
- Agent derives ALL paths from the slug; `user =` / `group =` in the pool conf
  stay the **real OS user** (privilege unchanged). This is exactly the
  `jabali-pma` opaque-instance precedent — `fpm-pre-start`/`fpm-exec` already
  derive the OS user from the pool conf's `user =` line, so **the shell shims
  need no change**.
  - pool conf: `/etc/php/<ver>/fpm/pool.d/jabali-<slug>.conf`
  - socket: `/run/php/jabali-<slug>/fpm.sock`
  - per-master fpm conf: `/etc/jabali-panel/fpm/<slug>.conf`
  - version pin: `/etc/jabali-panel/user-phpver/<slug>` (= `<ver>`)
  - systemd instance: `jabali-fpm@<slug>.service` + slice drop-in
- `globDeletePoolFiles` sibling-wipe: keep it ONLY for the default slug (its
  version changes in place). Versioned slugs encode their version → no wipe.
- **"Default version" is authoritative in the panel, not the agent.** The
  default = the user's existing pool version (the one at `slug=<user>`).

### Isolation caveat (documented, told to requester)

Buys **version + config** isolation, NOT uid isolation. All of a user's pools
still run as the same Unix uid/home. True per-site uid isolation remains the
one-user-per-site model. Stated on the issue.

### N×M cost

Pools default `pm_mode=ondemand` → idle version-pools spawn zero children, cost
is just the master process. No artificial cap needed.

### Orphan reaping

Last domain leaves version V → reconciler reaps that master (stop+disable+rm
via `php.pool.remove` by slug). Default pool is never reaped.

### CLI/shell version stays distinct

`/etc/jabali-panel/user-phpver/<user>` (no slug) drives `ensureUserCLIPHP` = the
user's SHELL php. Per-domain WEB version = the pool binding. Do not conflate.

## Waves

- **A — Agent (additive versioned master).** `php.pool.apply` takes `slug`;
  path derivation from slug; sibling-wipe gated to default slug;
  `php.pool.remove` by slug (stop+disable+rm). Unit tests. No behavior change
  when slug==user.
- **B — Panel model/repo/reconciler.** Repo `ListByUserID`. `reconcilePHPPools`:
  ensure default pool (as today) + apply every non-default pool with ≥1 bound
  domain (pending/error) + reap non-default pools with 0 bound domains. Socket
  resolver helper `poolSocketPath(pool, user)`.
- **C — Vhost.** `domain.create` params carry the bound pool's slug/socket;
  `fastcgi_pass` resolves per domain. Default domains unchanged. Contract test.
- **D — API + UI + CLI.** Domain PHP-version selector endpoint: find-or-create
  `(user, ver)` pool (status pending) + rebind `domain.php_pool_id` + schedule
  reconcile. UI dropdown in Domain PHP panel. CLI parity + golden.
- **E — ADRs + restore fix + runbook + VM E2E.** Amend ADR-0023 (lift the
  one-pool MVP clause), note ADR-0025 (slice hosts N masters/user). Fix backup
  restore apply path. Runbook. EICAR-style live check on a VM.

## Security: EOL PHP warning (operator decision, 2026-07-01)

The one real trade-off is that per-domain pinning makes it easy to leave a
public site on an EOL PHP. No hard block (operators legitimately need 7.4 for
legacy apps) — instead a visible **EOL warning**:

- **Primary:** admin PHP-versions/install page
  `panel-ui/src/shells/admin/php/VersionsTab.tsx` — a tooltip / `Tag`/`Alert`
  on any EOL version row ("EOL — no upstream security patches; running it on a
  public site is a security risk").
- **Secondary (Wave D):** the per-domain version selector shows the same EOL
  marker inline on EOL options.
- **Source of truth:** a small static EOL-date table (php.net supported
  versions) in one place, shared by both surfaces. `version <= lastEOL` (by
  date) => EOL. Keep it a plain constant; no network lookup.

No functional block — warning only. Default stays the latest stable.

## ADR amendments

- **ADR-0023 §Decision:** strike "each panel user gets exactly one pool (MVP
  constraint)"; replace with the additive per-version model. Add dated Amended
  note. (0023 already *promised* per-domain version selection — this finishes
  it.)
- **ADR-0025:** note the per-user slice now hosts one master per active version.
