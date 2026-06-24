=== Jabali Cache ===
Contributors: jabalipanel, apunker
Tags: cache, redis, object cache, performance, page cache
Requires at least: 5.6
Tested up to: 7.0
Requires PHP: 7.4
Stable tag: 1.0.0
License: GPLv2 or later
License URI: https://www.gnu.org/licenses/gpl-2.0.html

Redis-backed persistent object cache (and optional full-page cache) tuned for the Jabali hosting panel.

== Description ==

Jabali Cache turns WordPress's transient/object cache into a persistent Redis-backed cache using the Redis instance the Jabali panel already provisions. It is purpose-built for the panel's shared-hosting model:

* **Zero-config on Jabali.** Connects to the panel Redis over the unix socket `/run/redis/redis.sock`, logical database 1 (ADR-0059). No host/port to enter.
* **Per-site isolation.** Every install gets a unique key prefix. The shared Redis database is namespaced per site; one site can never read or flush another's cache.
* **LRU-safe.** The panel Redis runs `allkeys-lru`. Every read is best-effort, so an evicted key just falls through to the database — never an error.
* **Works without phpredis.** If the `redis` PHP extension is not installed (the Jabali default), the plugin uses a dependency-free pure-PHP RESP client over the socket. If phpredis is present, it is used automatically.
* **Fails safe.** If Redis is unreachable, WordPress keeps running with a normal per-request cache. The admin screen shows exactly why and how to fix it.

A full-page cache is included but **off by default**, because Jabali already provides an nginx fastcgi microcache at the edge (ADR-0108). Enable the WP page cache only if you turned the nginx one off.

== Installation ==

1. Upload the `jabali-cache` folder to `wp-content/plugins/`.
2. Activate **Jabali Cache** from Plugins. Activation installs the `object-cache.php` drop-in automatically.
3. Visit **Settings → Jabali Cache** to confirm "Redis connection: Connected".

If the status shows "Not reachable", the screen prints the exact host prerequisite (see FAQ).

== Frequently Asked Questions ==

= The status says Redis is not reachable. =

On the Jabali panel, socket access is provisioned automatically when you enable caching for the site (Applications → cache toggle). If the status still shows "Not reachable", re-run that toggle.

On a standalone host the site's PHP user needs two things:

1. **open_basedir** must allow the socket path — add `/run/redis` (or your socket's directory) to the pool's `open_basedir`.
2. **Read access to the socket** for the PHP-FPM user.

Until then the plugin runs as a non-persistent per-request cache and the site keeps working normally — caching just isn't accelerated.

= Does it need the phpredis extension? =

No. It auto-detects phpredis and uses it if present, otherwise a built-in pure-PHP client.

= Will it conflict with the nginx page cache? =

No. The object cache complements the nginx microcache. The WP page cache is off by default to avoid double-caching.

= Can I use it outside the Jabali panel? =

Yes. Point it at any Redis over a unix socket or TCP. Define overrides in `wp-config.php`:
`JABALI_CACHE_SOCKET`, or `JABALI_CACHE_HOST` + `JABALI_CACHE_PORT`, plus `JABALI_CACHE_DB`, `JABALI_CACHE_PASSWORD`, and `JABALI_CACHE_PREFIX`. With no overrides it defaults to the Jabali socket and derives a per-site key prefix.

== Changelog ==

= 1.0.0 =
* Initial release: Redis object cache, optional page cache, WP-CLI, admin diagnostics.

== Upgrade Notice ==

= 1.0.0 =
Initial release.
