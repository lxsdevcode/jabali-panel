<?php
/**
 * Plugin Name:       Jabali Cache
 * Plugin URI:        https://jabali-panel.com/
 * Description:        Redis-backed object cache (and optional full-page cache) tuned for the Jabali hosting panel. Uses the shared panel Redis over a unix socket with per-site key isolation. Works with or without the phpredis extension.
 * Version:           1.0.3
 * Requires at least: 5.6
 * Requires PHP:      7.4
 * Author:            Jabali Panel
 * License:           GPL-2.0-or-later
 * License URI:       https://www.gnu.org/licenses/gpl-2.0.html
 * Text Domain:       jabali-cache
 *
 * @package Jabali_Cache
 */

defined( 'ABSPATH' ) || exit;

if ( ! defined( 'JABALI_CACHE_VERSION' ) ) {
	define( 'JABALI_CACHE_VERSION', '1.0.3' );
}
if ( ! defined( 'JABALI_CACHE_PLUGIN_FILE' ) ) {
	define( 'JABALI_CACHE_PLUGIN_FILE', __FILE__ );
}
if ( ! defined( 'JABALI_CACHE_PLUGIN_DIR' ) ) {
	define( 'JABALI_CACHE_PLUGIN_DIR', __DIR__ );
}

require_once __DIR__ . '/includes/lib.php';
require_once __DIR__ . '/includes/class-settings.php';
require_once __DIR__ . '/includes/class-dropin-manager.php';
require_once __DIR__ . '/includes/class-page-cache.php';

/**
 * Activation: seed settings, write the runtime config file, and install the
 * wp-content drop-ins. Drop-in install is best-effort — if wp-content is not
 * writable, the admin screen offers a manual "Install / repair" button.
 */
function jabali_cache_activate() {
	$settings = Jabali_Cache_Settings::get();
	Jabali_Cache_Settings::save( $settings ); // normalises + derives prefix + writes config file.

	$mgr = new Jabali_Cache_Dropin_Manager( JABALI_CACHE_PLUGIN_DIR );
	$mgr->install();

	set_transient( 'jabali_cache_activated', 1, 60 );
}
register_activation_hook( __FILE__, 'jabali_cache_activate' );

/**
 * Deactivation: remove our drop-ins (so WordPress falls back to core caching)
 * and drop the generated config file. Settings in the options table are kept
 * so re-activating restores the previous configuration.
 */
function jabali_cache_deactivate() {
	$mgr = new Jabali_Cache_Dropin_Manager( JABALI_CACHE_PLUGIN_DIR );
	$mgr->remove();
	Jabali_Cache_Settings::delete_config_file();
}
register_deactivation_hook( __FILE__, 'jabali_cache_deactivate' );

/**
 * Admin wiring.
 */
function jabali_cache_boot_admin() {
	require_once JABALI_CACHE_PLUGIN_DIR . '/includes/class-admin.php';
	$admin = new Jabali_Cache_Admin( JABALI_CACHE_PLUGIN_FILE );
	$admin->hooks();

	// One-time post-activation nudge if wp-config.php lacks WP_CACHE (only
	// matters for the optional page cache).
	if ( get_transient( 'jabali_cache_activated' ) ) {
		delete_transient( 'jabali_cache_activated' );
	}
}
if ( is_admin() ) {
	add_action( 'init', 'jabali_cache_boot_admin' );
}

/**
 * Conditional-request validators (Last-Modified + ETag) on cacheable front-end
 * responses, so returning visitors + CDNs revalidate with a 304 instead of a
 * full re-download. Gated on caching being enabled; never on admin, logged-in,
 * non-GET, feeds, REST, or error pages. nginx caches these on a MISS and honours
 * If-Modified-Since on HITs (no PHP-side 304 short-circuit, to stay clear of
 * REST/feed edge cases).
 */
function jabali_cache_send_validators() {
	$cfg = Jabali_Cache_Config::load();
	if ( empty( $cfg['enabled'] ) ) {
		return;
	}
	if ( is_admin() || is_user_logged_in() || is_feed() || is_404() ) {
		return;
	}
	$method = isset( $_SERVER['REQUEST_METHOD'] ) ? $_SERVER['REQUEST_METHOD'] : 'GET';
	if ( 'GET' !== $method && 'HEAD' !== $method ) {
		return;
	}
	if ( defined( 'REST_REQUEST' ) && REST_REQUEST ) {
		return;
	}
	$lm = get_lastpostmodified( 'GMT' );
	if ( ! $lm ) {
		return;
	}
	$ts      = strtotime( $lm . ' GMT' );
	$lastmod = gmdate( 'D, d M Y H:i:s', $ts ) . ' GMT';
	$uri     = isset( $_SERVER['REQUEST_URI'] ) ? sanitize_text_field( wp_unslash( $_SERVER['REQUEST_URI'] ) ) : '/';
	$etag    = '"' . md5( $lastmod . '|' . $uri ) . '"';
	header( 'Last-Modified: ' . $lastmod );
	header( 'ETag: ' . $etag );
}
add_action( 'template_redirect', 'jabali_cache_send_validators', 9 );

/**
 * WP-CLI wiring.
 */
if ( defined( 'WP_CLI' ) && WP_CLI ) {
	require_once JABALI_CACHE_PLUGIN_DIR . '/includes/class-cli.php';
	WP_CLI::add_command( 'jabali-cache', new Jabali_Cache_CLI( JABALI_CACHE_PLUGIN_DIR ) );
}

/**
 * Page-cache invalidation. When content changes, purge the full-page cache so
 * visitors never see a stale page. Object cache is invalidated by WordPress
 * core's normal wp_cache_delete calls, so it needs no extra hooks here.
 */
function jabali_cache_register_purge_hooks() {
	// GH #603: gate on the SAME source Page_Cache::run() serves/stores from —
	// Jabali_Cache_Config (constants + defaults) — not the options table. When
	// page cache was enabled by constants but the option didn't match, pages
	// were cached and served while these purge hooks were never registered, so
	// a content change left stale pages until TTL expiry.
	$cfg = Jabali_Cache_Config::load();
	if ( empty( $cfg['enabled'] ) || empty( $cfg['page_cache'] ) ) {
		return;
	}
	$purge = 'jabali_cache_purge_pages';
	add_action( 'save_post', $purge );
	add_action( 'deleted_post', $purge );
	add_action( 'trashed_post', $purge );
	add_action( 'comment_post', $purge );
	add_action( 'edit_comment', $purge );
	add_action( 'wp_set_comment_status', $purge );
	add_action( 'switch_theme', $purge );
	add_action( 'customize_save_after', $purge );
	add_action( 'updated_option', 'jabali_cache_maybe_purge_on_option', 10, 1 );
}
add_action( 'init', 'jabali_cache_register_purge_hooks' );

/**
 * Purge every full-page cache entry for this site.
 */
function jabali_cache_purge_pages() {
	if ( ! class_exists( 'Jabali_Cache_Page_Cache' ) ) {
		return;
	}
	$pc = new Jabali_Cache_Page_Cache();
	$pc->purge_all();
}

/**
 * Purge pages when a content-affecting option changes (e.g. permalinks,
 * blogname, site URL).
 *
 * @param string $option
 */
function jabali_cache_maybe_purge_on_option( $option ) {
	static $watched = array( 'permalink_structure', 'blogname', 'home', 'siteurl', 'page_on_front', 'show_on_front' );
	if ( in_array( $option, $watched, true ) ) {
		jabali_cache_purge_pages();
	}
}
