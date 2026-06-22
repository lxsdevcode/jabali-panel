# Jabali WP Cache

Redis-backed WordPress **object cache** (and optional full-page cache) built for the
[Jabali hosting panel](https://jabali-panel.com/). It plugs into the Redis instance the
panel already provisions and follows the panel's shared-hosting security model.

> This is a self-contained WordPress plugin. It does **not** modify any Jabali panel
> code — it lives under `wp-plugins/jabali-wp-cache/` and is deployed into a WordPress
> install's `wp-content/plugins/` directory.

## Why it fits Jabali

| Jabali fact | How the plugin uses it |
|---|---|
| Redis at `unix:///run/redis/redis.sock`, **db 1 reserved for WP** (ADR-0059) | Default connection target; no host/port config needed. |
| Redis runs `maxmemory-policy allkeys-lru` | Every read is best-effort; an evicted key is a normal miss, never an error. |
| db 1 is **shared across all tenants** | Isolation is by per-site key **prefix**. The plugin **never** issues `FLUSHDB`; flushes are prefix-scoped via `SCAN` + `DEL`. |
| Jabali does **not** install the `redis` PHP extension | Ships a dependency-free pure-PHP RESP client; uses phpredis automatically if present. |
| Tenant PHP-FPM runs as the site user, `open_basedir`-jailed | Connects gracefully; on failure it degrades to a per-request cache and prints the exact host fix. |
| nginx fastcgi microcache already exists (ADR-0108) | WP full-page cache is **off by default** to avoid double-caching; object cache is the primary win. |

## Architecture

```
jabali-wp-cache/
├── jabali-wp-cache.php              # plugin bootstrap, activation, purge hooks
├── uninstall.php                 # prefix-scoped Redis purge + cleanup
├── config-sample.php             # optional wp-config.php overrides
├── includes/
│   ├── lib.php                   # Config resolver + Redis client (phpredis OR pure-PHP RESP)
│   ├── class-object-cache.php    # WP_Object_Cache engine
│   ├── class-page-cache.php      # full-page cache engine
│   ├── class-settings.php        # options store + generated config file writer
│   ├── class-dropin-manager.php  # installs/removes wp-content drop-ins (signature-guarded)
│   ├── class-admin.php           # Settings → Jabali WP Cache screen + diagnostics
│   └── class-cli.php             # `wp jabali-wp-cache ...`
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

1. Copy `jabali-wp-cache/` into `wp-content/plugins/`.
2. Activate it — the `object-cache.php` drop-in is installed automatically.
3. **Settings → Jabali WP Cache** → confirm *Redis connection: Connected*.

### Host prerequisites (panel admin, one-time)

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
wp jabali-wp-cache status          # connectivity, driver, key count
wp jabali-wp-cache diagnose        # extension + connection hints
wp jabali-wp-cache flush [--pages] # flush object cache (+ page cache)
wp jabali-wp-cache enable|disable
wp jabali-wp-cache update-dropins  # (re)install drop-ins
wp jabali-wp-cache remove-dropins
```

## Configuration

Everything is configurable from the settings screen. Power users can override via
constants in `wp-config.php` — see `config-sample.php`. Constants win over the screen.

## Tests

`tests/test-lib.php` is a plain-PHP harness (no PHPUnit needed):

```bash
php wp-plugins/jabali-wp-cache/tests/test-lib.php
# add a live socket round-trip when Redis is available:
JABALI_TEST_REDIS=/run/redis/redis.sock php wp-plugins/jabali-wp-cache/tests/test-lib.php
```

## License

GPL-2.0-or-later.
