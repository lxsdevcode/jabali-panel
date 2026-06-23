<?php
/**
 * Plugin Name: Jabali Cache — Advanced Cache Drop-in
 * Description: Redis-backed full-page cache for the jabali panel. Installed by the Jabali Cache plugin; do not edit by hand.
 * Version: 1.0.0
 *
 * Copied to wp-content/advanced-cache.php. WordPress includes this only when
 * the WP_CACHE constant is true (wp-config.php). It runs before the rest of
 * WordPress, so it must be self-locating and must never fatal: any problem
 * just means the page is generated normally.
 *
 * @package Jabali_Cache
 */

defined( 'ABSPATH' ) || exit;

// advanced-cache must not interfere with the installer or CLI.
if ( ( defined( 'WP_INSTALLING' ) && WP_INSTALLING ) || ( defined( 'WP_CLI' ) && WP_CLI ) ) {
	return;
}

if ( ! defined( 'JABALI_CACHE_PLUGIN_DIR' ) ) {
	$jabali_cache_stamped = '__JABALI_CACHE_PLUGIN_DIR__';
	if ( 0 !== strpos( $jabali_cache_stamped, '__JABALI' ) && is_dir( $jabali_cache_stamped ) ) {
		define( 'JABALI_CACHE_PLUGIN_DIR', $jabali_cache_stamped );
	} elseif ( defined( 'WP_PLUGIN_DIR' ) && is_dir( WP_PLUGIN_DIR . '/jabali-cache' ) ) {
		define( 'JABALI_CACHE_PLUGIN_DIR', WP_PLUGIN_DIR . '/jabali-cache' );
	} elseif ( defined( 'WP_CONTENT_DIR' ) && is_dir( WP_CONTENT_DIR . '/plugins/jabali-cache' ) ) {
		define( 'JABALI_CACHE_PLUGIN_DIR', WP_CONTENT_DIR . '/plugins/jabali-cache' );
	} else {
		define( 'JABALI_CACHE_PLUGIN_DIR', '' );
	}
}

if ( '' === JABALI_CACHE_PLUGIN_DIR ) {
	return;
}

$jabali_cache_page_engine = JABALI_CACHE_PLUGIN_DIR . '/includes/class-page-cache.php';
if ( ! is_readable( $jabali_cache_page_engine ) ) {
	return;
}

require_once JABALI_CACHE_PLUGIN_DIR . '/includes/lib.php';
require_once $jabali_cache_page_engine;

try {
	$jabali_cache_page_cache = new Jabali_Cache_Page_Cache();
	$jabali_cache_page_cache->run();
} catch ( \Throwable $e ) {
	// Never let the page cache take the site down.
	return;
}
