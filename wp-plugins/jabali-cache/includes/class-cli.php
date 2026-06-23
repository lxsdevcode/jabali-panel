<?php
/**
 * Jabali Cache — WP-CLI commands.
 *
 *   wp jabali-cache status
 *   wp jabali-cache enable
 *   wp jabali-cache disable
 *   wp jabali-cache flush [--pages]
 *   wp jabali-cache update-dropins
 *   wp jabali-cache remove-dropins
 *   wp jabali-cache diagnose
 *
 * @package Jabali_Cache
 */

defined( 'ABSPATH' ) || exit;

if ( ! defined( 'WP_CLI' ) || ! WP_CLI ) {
	return;
}

class Jabali_Cache_CLI {

	/** @var string */
	private $plugin_dir;

	public function __construct( $plugin_dir = '' ) {
		$this->plugin_dir = $plugin_dir ? $plugin_dir : dirname( __DIR__ );
	}

	/**
	 * Show cache status and live Redis connectivity.
	 *
	 * @when after_wp_load
	 */
	public function status() {
		$s   = Jabali_Cache_Settings::get();
		$cfg = Jabali_Cache_Config::load();
		$c   = new Jabali_Cache_Client( $cfg );
		$ok  = $c->connect();

		$rows = array(
			array( 'field' => 'enabled', 'value' => $s['enabled'] ? 'yes' : 'no' ),
			array( 'field' => 'connected', 'value' => $ok ? 'yes' : 'no' ),
			array( 'field' => 'driver', 'value' => $ok ? $c->driver() : '-' ),
			array( 'field' => 'target', 'value' => ( 'unix' === $cfg['scheme'] ? $cfg['socket'] : $cfg['host'] . ':' . $cfg['port'] ) ),
			array( 'field' => 'database', 'value' => (string) $cfg['database'] ),
			array( 'field' => 'prefix', 'value' => $cfg['prefix'] ),
			array( 'field' => 'keys', 'value' => $ok ? (string) $c->count_keys( $cfg['prefix'] ) : '-' ),
			array( 'field' => 'page_cache', 'value' => $s['page_cache'] ? 'on' : 'off' ),
		);
		if ( ! $ok ) {
			$rows[] = array( 'field' => 'error', 'value' => $c->last_error() );
		}
		$c->close();
		\WP_CLI\Utils\format_items( 'table', $rows, array( 'field', 'value' ) );
	}

	/**
	 * Enable caching.
	 *
	 * @when after_wp_load
	 */
	public function enable() {
		$s            = Jabali_Cache_Settings::get();
		$s['enabled'] = true;
		Jabali_Cache_Settings::save( $s );
		$this->ensure_dropins();
		\WP_CLI::success( 'Caching enabled.' );
	}

	/**
	 * Disable caching (settings kept; drop-ins left in place but inert).
	 *
	 * @when after_wp_load
	 */
	public function disable() {
		$s            = Jabali_Cache_Settings::get();
		$s['enabled'] = false;
		Jabali_Cache_Settings::save( $s );
		if ( function_exists( 'wp_cache_flush' ) ) {
			wp_cache_flush();
		}
		\WP_CLI::success( 'Caching disabled.' );
	}

	/**
	 * Flush the cache.
	 *
	 * [--pages]
	 * : Also purge full-page cache entries.
	 *
	 * @when after_wp_load
	 */
	public function flush( $args, $assoc ) {
		if ( function_exists( 'wp_cache_flush' ) ) {
			wp_cache_flush();
		}
		if ( isset( $assoc['pages'] ) && class_exists( 'Jabali_Cache_Page_Cache' ) ) {
			$pc = new Jabali_Cache_Page_Cache();
			$n  = $pc->purge_all();
			\WP_CLI::log( "Purged {$n} page cache entries." );
		}
		\WP_CLI::success( 'Cache flushed.' );
	}

	/**
	 * Install or repair the wp-content drop-ins.
	 *
	 * @when after_wp_load
	 */
	public function update_dropins() {
		$res = $this->ensure_dropins();
		if ( $res['object'] ) {
			\WP_CLI::success( 'Drop-ins installed/updated.' );
		} else {
			\WP_CLI::error( 'Could not install object-cache.php (a foreign drop-in may be present, or wp-content is not writable).' );
		}
	}

	/**
	 * Remove the Jabali drop-ins.
	 *
	 * @when after_wp_load
	 */
	public function remove_dropins() {
		$mgr = new Jabali_Cache_Dropin_Manager( $this->plugin_dir );
		$mgr->remove();
		\WP_CLI::success( 'Drop-ins removed.' );
	}

	/**
	 * Diagnose connectivity and print an actionable hint on failure.
	 *
	 * @when after_wp_load
	 */
	public function diagnose() {
		$cfg = Jabali_Cache_Config::load();
		\WP_CLI::log( 'phpredis extension: ' . ( class_exists( 'Redis' ) ? 'present' : 'absent (using pure-PHP client)' ) );
		\WP_CLI::log( 'igbinary extension: ' . ( function_exists( 'igbinary_serialize' ) ? 'present' : 'absent' ) );
		\WP_CLI::log( 'scheme/target: ' . $cfg['scheme'] . ' ' . ( 'unix' === $cfg['scheme'] ? $cfg['socket'] : $cfg['host'] . ':' . $cfg['port'] ) );

		$c = new Jabali_Cache_Client( $cfg );
		if ( $c->connect() ) {
			\WP_CLI::success( 'Connected via ' . $c->driver() . '. Keys for this site: ' . $c->count_keys( $cfg['prefix'] ) );
			$c->close();
			return;
		}
		\WP_CLI::warning( 'Not connected: ' . $c->last_error() );
		\WP_CLI::log( 'Hints:' );
		\WP_CLI::log( '  - open_basedir must include /run/redis (or the socket path).' );
		\WP_CLI::log( '  - the PHP-FPM system user needs read/write on the Redis socket (the Jabali panel grants this automatically).' );
		\WP_CLI::log( '  - redis-server must be running on the panel host.' );
	}

	private function ensure_dropins() {
		$mgr = new Jabali_Cache_Dropin_Manager( $this->plugin_dir );
		return $mgr->install();
	}
}
