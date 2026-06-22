=== Jabali WP Cache ===
Contributors: jabalipanel
Tags: cache, redis, object cache, performance, page cache
Requires at least: 5.6
Tested up to: 6.7
Requires PHP: 7.4
Stable tag: 1.0.0
License: GPLv2 or later
License URI: https://www.gnu.org/licenses/gpl-2.0.html

Redis-backed persistent object cache (and optional full-page cache) tuned for the Jabali hosting panel.

== Description ==

Jabali WP Cache turns WordPress's transient/object cache into a persistent Redis-backed cache using the Redis instance the Jabali panel already provisions. It is purpose-built for the panel's shared-hosting model:

* **Zero-config on Jabali.** Connects to the panel Redis over the unix socket `/run/redis/redis.sock`, logical database 1 (ADR-0059). No host/port to enter.
* **Per-site isolation.** Every install gets a unique key prefix. The shared Redis database is namespaced per site; one site can never read or flush another's cache.
* **LRU-safe.** The panel Redis runs `allkeys-lru`. Every read is best-effort, so an evicted key just falls through to the database — never an error.
* **Works without phpredis.** If the `redis` PHP extension is not installed (the Jabali default), the plugin uses a dependency-free pure-PHP RESP client over the socket. If phpredis is present, it is used automatically.
* **Fails safe.** If Redis is unreachable, WordPress keeps running with a normal per-request cache. The admin screen shows exactly why and how to fix it.

A full-page cache is included but **off by default**, because Jabali already provides an nginx fastcgi microcache at the edge (ADR-0108). Enable the WP page cache only if you turned the nginx one off.

== Installation ==

1. Upload the `jabali-wp-cache` folder to `wp-content/plugins/`.
2. Activate **Jabali WP Cache** from Plugins. Activation installs the `object-cache.php` drop-in automatically.
3. Visit **Settings → Jabali WP Cache** to confirm "Redis connection: Connected".

If the status shows "Not reachable", the screen prints the exact host prerequisite (see FAQ).

== Frequently Asked Questions ==

= The status says Redis is not reachable. =

On a default Jabali host the tenant PHP-FPM pool cannot open the shared Redis socket until two host-level prerequisites are met (panel admin):

1. **open_basedir** must allow the socket. Add `/run/redis` to the pool's `open_basedir`.
2. **Group membership.** The site's system user must be in the `jabali-sockets` group (`usermod -aG jabali-sockets <user>` then restart the pool), so the `0660` socket is reachable.

Until then the plugin runs as a non-persistent cache and the site works normally.

= Does it need the phpredis extension? =

No. It auto-detects phpredis and uses it if present, otherwise a built-in pure-PHP client.

= Will it conflict with the nginx page cache? =

No. The object cache complements the nginx microcache. The WP page cache is off by default to avoid double-caching.

== Changelog ==

= 1.0.0 =
* Initial release: Redis object cache, optional page cache, WP-CLI, admin diagnostics.
