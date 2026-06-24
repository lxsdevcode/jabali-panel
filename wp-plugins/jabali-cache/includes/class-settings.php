<?php
/**
 * Jabali Cache — settings store and config-file writer.
 *
 * Settings live in a single WP option. On every change we also (re)write
 * wp-content/jabali-cache-config.php — a plain PHP array file the drop-ins
 * read before WordPress (and thus the options table) is available.
 *
 * @package Jabali_Cache
 */

defined( 'ABSPATH' ) || exit;

if ( ! class_exists( 'Jabali_Cache_Client' ) ) {
	require_once __DIR__ . '/lib.php';
}

class Jabali_Cache_Settings {

	const OPTION = 'jabali_cache_settings';

	/**
	 * @return array<string,mixed>
	 */
	public static function defaults() {
		return array(
			'enabled'     => true,
			'page_cache'  => false,
			'page_ttl'    => 300,
			'maxttl'      => 0,
			'scheme'      => 'unix',
			'socket'      => '/run/redis/redis.sock',
			'host'        => '127.0.0.1',
			'port'        => 6379,
			'database'    => 1,
			'password'    => '',
			'prefix'      => '', // auto-derived on first save.
		);
	}

	/**
	 * @return array<string,mixed>
	 */
	public static function get() {
		$saved = get_option( self::OPTION, array() );
		if ( ! is_array( $saved ) ) {
			$saved = array();
		}
		return array_merge( self::defaults(), $saved );
	}

	/**
	 * Persist settings (validated) and regenerate the drop-in config file.
	 *
	 * @param array<string,mixed> $input
	 * @return array<string,mixed> the stored settings.
	 */
	public static function save( array $input ) {
		$cur = self::get();
		$out = $cur;

		$out['enabled']    = ! empty( $input['enabled'] );
		$out['page_cache'] = ! empty( $input['page_cache'] );
		$out['page_ttl']   = isset( $input['page_ttl'] ) ? max( 1, (int) $input['page_ttl'] ) : $cur['page_ttl'];
		$out['maxttl']     = isset( $input['maxttl'] ) ? max( 0, (int) $input['maxttl'] ) : $cur['maxttl'];

		if ( isset( $input['scheme'] ) && in_array( $input['scheme'], array( 'unix', 'tcp' ), true ) ) {
			$out['scheme'] = $input['scheme'];
		}
		if ( isset( $input['socket'] ) ) {
			$out['socket'] = self::clean_scalar( $input['socket'] );
		}
		if ( isset( $input['host'] ) ) {
			$out['host'] = self::clean_scalar( $input['host'] );
		}
		if ( isset( $input['port'] ) ) {
			$out['port'] = max( 1, min( 65535, (int) $input['port'] ) );
		}
		if ( isset( $input['database'] ) ) {
			$out['database'] = max( 0, min( 15, (int) $input['database'] ) );
		}
		if ( isset( $input['password'] ) ) {
			$out['password'] = self::clean_scalar( $input['password'] );
		}

		// Stable per-site prefix: derive once, then keep it.
		if ( empty( $cur['prefix'] ) ) {
			$out['prefix'] = self::derive_prefix();
		}

		update_option( self::OPTION, $out, false );
		// Settings persist in the options table only (wp.org compliance). Older
		// versions also wrote wp-content/jabali-cache-config.php; remove it so a
		// stale executable config file can't shadow the option.
		self::delete_config_file();
		Jabali_Cache_Config::reset();
		return $out;
	}


	public static function delete_config_file() {
		$path = Jabali_Cache_Config::config_file_path();
		if ( file_exists( $path ) ) {
			@unlink( $path ); // phpcs:ignore
		}
	}

	private static function clean_scalar( $v ) {
		$v = (string) $v;
		// Strip control chars; these values flow into a generated PHP file and
		// into Redis connection calls.
		return preg_replace( '/[\x00-\x1f\x7f]/', '', $v );
	}

	private static function derive_prefix() {
		$seed = '';
		if ( defined( 'ABSPATH' ) ) {
			$seed .= ABSPATH;
		}
		if ( defined( 'DB_NAME' ) ) {
			$seed .= '|' . DB_NAME;
		}
		$home = function_exists( 'home_url' ) ? home_url() : '';
		$seed .= '|' . $home;
		if ( '' === $seed ) {
			$seed = __FILE__;
		}
		return substr( md5( $seed ), 0, 12 );
	}
}
