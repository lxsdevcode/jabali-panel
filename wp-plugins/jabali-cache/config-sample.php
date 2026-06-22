<?php
/**
 * Jabali Cache — sample wp-config.php constants.
 *
 * You normally do NOT need any of these: the plugin auto-detects the jabali
 * panel Redis (unix socket /run/redis/redis.sock, database 1) and writes its
 * own wp-content/jabali-cache-config.php from the settings screen.
 *
 * Define any of these in wp-config.php (ABOVE the "stop editing" line) only to
 * override the defaults. Constants take precedence over the settings screen.
 *
 * @package Jabali_Cache
 */

// Master kill switch (useful for staging): true disables all caching.
// define( 'JABALI_CACHE_DISABLED', false );

// Connection — unix socket is the jabali default and recommended path.
// define( 'JABALI_CACHE_SOCKET', '/run/redis/redis.sock' );
// define( 'JABALI_CACHE_DB', 1 );          // ADR-0059: database 1 reserved for WP.
// define( 'JABALI_CACHE_PASSWORD', '' );    // the panel socket has no AUTH.

// TCP fallback (only if you run a non-jabali Redis). Setting a host switches
// the scheme to tcp automatically.
// define( 'JABALI_CACHE_HOST', '127.0.0.1' );
// define( 'JABALI_CACHE_PORT', 6379 );

// Explicit per-site key prefix. Leave unset to auto-derive a stable, unique
// prefix from the site URL + path (recommended on shared Redis).
// define( 'JABALI_CACHE_PREFIX', 'mysite' );

// Cap object-cache key TTL in seconds (0 = no expiry; rely on Redis LRU).
// define( 'JABALI_CACHE_MAXTTL', 0 );

// Enable the OPTIONAL full-page cache. Also requires WP_CACHE below.
// NOTE: jabali already ships an nginx fastcgi microcache (ADR-0108); only
// enable the WP page cache if you disabled that at the panel.
// define( 'WP_CACHE', true );
