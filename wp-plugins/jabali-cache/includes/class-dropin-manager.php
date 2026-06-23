<?php
/**
 * Jabali Cache — drop-in manager.
 *
 * Installs / updates / removes the wp-content/object-cache.php and
 * advanced-cache.php drop-ins, stamping the real plugin path into each so the
 * drop-ins can locate the engine no matter where wp-content lives.
 *
 * Only ever touches files it owns: it verifies the "Jabali Cache" signature
 * before overwriting or deleting an existing drop-in, so a third-party object
 * cache is never clobbered.
 *
 * @package Jabali_Cache
 */

defined( 'ABSPATH' ) || exit;

class Jabali_Cache_Dropin_Manager {

	const SIGNATURE = 'Jabali Cache —';

	/** @var string */
	private $plugin_dir;

	/** @var string */
	private $content_dir;

	public function __construct( $plugin_dir ) {
		$this->plugin_dir  = rtrim( $plugin_dir, '/' );
		$this->content_dir = defined( 'WP_CONTENT_DIR' ) ? WP_CONTENT_DIR : ABSPATH . 'wp-content';
	}

	/**
	 * @return array{object:bool,advanced:bool}
	 */
	public function install() {
		return array(
			'object'   => $this->copy_dropin( 'object-cache.php' ),
			'advanced' => $this->copy_dropin( 'advanced-cache.php' ),
		);
	}

	/**
	 * Remove only our drop-ins (verified by signature).
	 */
	public function remove() {
		foreach ( array( 'object-cache.php', 'advanced-cache.php' ) as $name ) {
			$dest = $this->content_dir . '/' . $name;
			if ( $this->is_ours( $dest ) ) {
				@unlink( $dest ); // phpcs:ignore
			}
		}
	}

	/**
	 * @param string $name object-cache.php | advanced-cache.php
	 * @return bool
	 */
	private function copy_dropin( $name ) {
		$src  = $this->plugin_dir . '/dropins/' . $name;
		$dest = $this->content_dir . '/' . $name;

		if ( ! is_readable( $src ) ) {
			return false;
		}
		// Refuse to overwrite a foreign drop-in.
		if ( file_exists( $dest ) && ! $this->is_ours( $dest ) ) {
			return false;
		}

		$code = file_get_contents( $src ); // phpcs:ignore WordPress.WP.AlternativeFunctions.file_get_contents_file_get_contents
		if ( false === $code ) {
			return false;
		}
		// Stamp the absolute plugin path so the drop-in finds the engine.
		$code = str_replace( '__JABALI_CACHE_PLUGIN_DIR__', $this->plugin_dir, $code );

		$tmp = $dest . '.tmp' . wp_rand( 1000, 9999 );
		if ( false === file_put_contents( $tmp, $code, LOCK_EX ) ) { // phpcs:ignore WordPress.WP.AlternativeFunctions.file_system_operations_file_put_contents
			return false;
		}
		@chmod( $tmp, 0644 ); // phpcs:ignore
		if ( ! @rename( $tmp, $dest ) ) { // phpcs:ignore
			@unlink( $tmp ); // phpcs:ignore
			return false;
		}
		return true;
	}

	/**
	 * @param string $path
	 * @return bool true if the file exists and carries our signature.
	 */
	public function is_ours( $path ) {
		if ( ! file_exists( $path ) ) {
			return false;
		}
		$head = file_get_contents( $path, false, null, 0, 512 ); // phpcs:ignore WordPress.WP.AlternativeFunctions.file_get_contents_file_get_contents
		return ( false !== $head && false !== strpos( $head, self::SIGNATURE ) );
	}

	/**
	 * @return array{object_installed:bool,object_ours:bool,object_foreign:bool,advanced_installed:bool,advanced_ours:bool,wp_cache_const:bool}
	 */
	public function status() {
		$obj = $this->content_dir . '/object-cache.php';
		$adv = $this->content_dir . '/advanced-cache.php';
		return array(
			'object_installed'   => file_exists( $obj ),
			'object_ours'        => $this->is_ours( $obj ),
			'object_foreign'     => file_exists( $obj ) && ! $this->is_ours( $obj ),
			'advanced_installed' => file_exists( $adv ),
			'advanced_ours'      => $this->is_ours( $adv ),
			'wp_cache_const'     => defined( 'WP_CACHE' ) && WP_CACHE,
		);
	}
}
