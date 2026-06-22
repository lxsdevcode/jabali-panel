# WordPress Cache switch — embed jabali-wp-cache + per-app toggle

**Status:** BLUEPRINT. Plugin embedded in repo (`9dd6aad7`); integration not built.
**Asks (user, 2026-06-23):**
1. Embed the `jabali-wp-cache` plugin in the panel. ✅ (merged to `wp-plugins/`).
2. Applications: a per-WordPress-app **switch** to add/remove the cache plugin.
3. Turning the switch ON **also enables the nginx page cache** for that app's domain.

## What the plugin is

`wp-plugins/jabali-wp-cache/` (another agent, reviewed): a WordPress Redis
object-cache + short-TTL page-cache. Configured by `define()` constants in a
config file (`JABALI_CACHE_SOCKET=/run/redis/redis.sock`, `JABALI_CACHE_DB=1`,
optional `JABALI_CACHE_PREFIX`). Per-site key **prefix** isolation; flushes only
its own prefix, never `FLUSHDB`. Admin actions are nonce + `manage_options`
gated; drop-in writes are atomic. No eval/exec.

## CRITICAL open decision — multi-tenant Redis isolation

install.sh provisions ONE unix-socket Redis (`/run/redis/redis.sock`, DB 1
"reserved for WP") with **no AUTH**. If the plugin (running as the tenant uid)
can connect to that socket, isolation between tenants is **logical only** (the
per-site key prefix) — a tenant who reaches the socket and knows/guesses another
site's prefix can read its cached objects (which can include transients holding
session-ish data). Before shipping the switch, pick one:

- **(A) Accept prefix-only isolation**, and verify the socket is readable by
  tenants by design. Cheapest; document the trust boundary (cache contents are
  not a hard secret; same-host tenants already share MariaDB instance, etc.).
- **(B) Per-tenant Redis ACL** (Redis 6+ `ACL SETUSER` with a key-pattern
  restriction `~<prefix>*` + `+@all -flushall -flushdb`), credential handed to
  the plugin via `JABALI_CACHE_PASSWORD`. Real enforcement; more moving parts
  (ACL lifecycle per user, socket must still be reachable).
- **(C) Per-tenant Redis DB number** — weak (DB count is capped at 16, no real
  isolation; a tenant can `SELECT` another DB). Reject.

Recommend **(B)** if the socket is tenant-reachable, else **(A)** with the socket
locked so only the panel + an explicit jabali-redis group (that tenants are NOT
in by default for the cache) — but then the plugin can't connect. **This is the
gating decision; do not build the switch until it's made.** Check the live socket
perms first (`stat /run/redis/redis.sock`, group membership).

## Design (once isolation is decided)

**Bundle.** install.sh deploys `wp-plugins/jabali-wp-cache/` →
`/usr/local/share/jabali/wp-plugins/jabali-wp-cache/` (root:root 0755, read-only
to tenants). `jabali update` re-syncs it. The agent installs FROM that path so
tenants never supply plugin code.

**DB.** `application_installs.cache_enabled BOOL NOT NULL DEFAULT 0` (migration).

**Agent** `wordpress.cache_set {install_path, os_user, enable, redis_socket, redis_db, prefix, redis_password?}`:
- enable: as the tenant (`buildSystemdRunCmd`), `wp plugin install
  /usr/local/share/jabali/wp-plugins/jabali-wp-cache --activate --path=<install>`;
  write the config file with the socket/db/prefix (+ password for option B);
  install the drop-ins (the plugin's own admin/CLI does this — prefer
  `wp jabali-cache enable` via its `class-cli.php` so the panel doesn't hand-place
  drop-ins). Idempotent.
- disable: `wp jabali-cache disable` (removes drop-ins) + `wp plugin deactivate
  jabali-wp-cache`. Leave the plugin files (cheap) or delete — decide.

**REST** `PUT /api/v1/applications/:id/cache {enabled}` (owner-or-admin scoped,
`write:apps`): set `cache_enabled`, dispatch `wordpress.cache_set`, AND on enable
flip the nginx page cache for the app's domain (reuse the existing
`PUT /domains/:id/cache {enabled:true}` path / its repo method) — on disable,
turn it back off. Only valid for `app_type=wordpress`.

**UI.** Applications list/detail: a Switch on each WordPress app → calls the REST;
optimistic + toast. Copy: "Caching (Redis object cache + nginx page cache)".

**nginx coupling.** The switch is the single control; it drives BOTH the WP plugin
and `domains.page_cache_enabled`. Document that the More-menu domain Caching toggle
and this app switch both touch the same nginx flag (last-writer-wins; the app
switch is the WP-aware front door).

## Phasing

0. **Decide Redis isolation (A/B).** Blocking. Check socket perms live.
1. Bundle in install.sh + DB migration.
2. Agent `wordpress.cache_set` (+ live-verify against a real WP install on the box:
   plugin active, object-cache drop-in present, `wp cache` works, keys land in
   Redis under the site prefix).
3. REST + nginx-cache coupling.
4. UI switch.
5. Verify end-to-end on a WP app; ADR for the isolation decision.

## Notes
- Only `app_type=wordpress` installs get the switch.
- The plugin's own admin page still works inside wp-admin; the panel switch is the
  jabali-managed front door (install + configure + nginx).
