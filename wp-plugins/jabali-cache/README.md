# Jabali Cache

Redis-backed WordPress **object cache** (and optional full-page cache) built for the
[Jabali hosting panel](https://jabali-panel.com/). It plugs into the Redis instance the
panel already provisions and follows the panel's shared-hosting security model.

> This is a self-contained WordPress plugin. It does **not** modify any Jabali panel
> code — it lives under `wp-plugins/jabali-cache/` and is deployed into a WordPress
> install's `wp-content/plugins/` directory.

## Why it fits Jabali

| Jabali fact | How the plugin uses it |
|---|---|
| Redis at `unix:///run/redis/redis.sock`, **db 1 reserved for WP** (ADR-0059) | Default connection target; no host/port config needed. |
| Redis runs `maxmemory-policy allkeys-lru` | Every read is best-effort; an evicted key is a normal miss, never an error. |
| db 1 is **shared across all tenants** | Isolation is by per-site key **prefix**. The plugin **never** issues `FLUSHDB`; flushes are prefix-scoped via `SCAN` + `DEL`. |
| Jabali installs the `redis` (phpredis) + `igbinary` PHP extensions by default (GH #606) | Uses phpredis with persistent `pconnect()`; the dependency-free pure-PHP RESP client is a portable fallback if the extension is ever absent. |
| Tenant PHP-FPM runs as the site user, `open_basedir`-jailed | Connects gracefully; on failure it degrades to a per-request cache and prints the exact host fix. |
| nginx fastcgi microcache already exists (ADR-0108) | WP full-page cache is **off by default** to avoid double-caching; object cache is the primary win. |

## Architecture

```
jabali-cache/
├── jabali-cache.php              # plugin bootstrap, activation, purge hooks
├── uninstall.php                 # prefix-scoped Redis purge + cleanup
├── config-sample.php             # optional wp-config.php overrides
├── includes/
│   ├── lib.php                   # Config resolver + Redis client (phpredis OR pure-PHP RESP)
│   ├── class-object-cache.php    # WP_Object_Cache engine
│   ├── class-page-cache.php      # full-page cache engine
│   ├── class-settings.php        # options store (legacy generated config file is deleted on save)
│   ├── class-dropin-manager.php  # installs/removes wp-content drop-ins (signature-guarded)
│   ├── class-admin.php           # Settings → Jabali Cache screen + diagnostics
│   └── class-cli.php             # `wp jabali-cache ...`
├── dropins/
│   ├── object-cache.php          # → wp-content/object-cache.php (thin loader, no-op fallback)
│   └── advanced-cache.php        # → wp-content/advanced-cache.php (page cache, fail-safe)
└── tests/
    └── test-lib.php              # runnable unit test for the RESP client + key building
```

The drop-ins are **thin loaders**: they locate the plugin (path stamped at install,
with fallbacks) and delegate to `includes/`. If the plugin folder is ever missing,
`object-cache.php` falls back to a per-request non-persistent cache so the site never
fatals.

## Install

1. Copy `jabali-cache/` into `wp-content/plugins/`.
2. Activate it — the `object-cache.php` drop-in is installed automatically.
3. **Settings → Jabali Cache** → confirm *Redis connection: Connected*.

### Host prerequisites (panel admin, one-time)

The panel performs these steps automatically during WordPress install. The
manual commands below are for reference / non-panel setups.

The shared Redis socket is `0660`, group `jabali-sockets`, and the tenant pool is
`open_basedir`-jailed. For a tenant site to reach it:

```bash
# 1) allow the socket path in the pool's open_basedir (php_admin_value[open_basedir])
#    add: /run/redis
# 2) put the site user in the socket group
usermod -aG jabali-sockets <site-user>
systemctl restart jabali-fpm@<site-user>.service
```

Without these the plugin still works — just as a non-persistent cache — and the admin
screen tells you which step is missing.

> These are **operator** steps, documented here only. This plugin does not change any
> panel install scripts.

## WP-CLI

```bash
wp jabali-cache status          # connectivity, driver, key count
wp jabali-cache diagnose        # extension + connection hints
wp jabali-cache flush [--pages] # flush object cache (+ page cache)
wp jabali-cache enable|disable
wp jabali-cache update-dropins  # (re)install drop-ins
wp jabali-cache remove-dropins
```

## Configuration

Everything is configurable from the settings screen. Power users can override via
constants in `wp-config.php` — see `config-sample.php`. Constants win over the screen.

## Tests

`tests/test-lib.php` is a plain-PHP harness (no PHPUnit needed):

```bash
php wp-plugins/jabali-cache/tests/test-lib.php
# add a live socket round-trip when Redis is available:
JABALI_TEST_REDIS=/run/redis/redis.sock php wp-plugins/jabali-cache/tests/test-lib.php
```

## License

GPL-2.0-or-later.
