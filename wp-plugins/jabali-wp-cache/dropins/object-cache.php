<?php
/**
 * Plugin Name: Jabali WP Cache — Object Cache Drop-in
 * Description: Redis-backed persistent object cache for the jabali panel. Installed by the Jabali WP Cache plugin; do not edit by hand.
 * Version: 1.0.0
 *
 * This file is copied to wp-content/object-cache.php. WordPress loads it very
 * early (before plugins), so it is responsible for defining every wp_cache_*
 * function. It delegates to Jabali_Cache_Object_Cache from the plugin.
 *
 * If the plugin directory is missing or unreadable, it transparently falls
 * back to a per-request, non-persistent cache so the site never breaks.
 *
 * @package Jabali_Cache
 */

defined( 'ABSPATH' ) || exit;

if ( ! defined( 'JABALI_CACHE_PLUGIN_DIR' ) ) {
	// Stamped by the plugin at install time; the placeholder below is the
	// fallback for manual installs.
	$jabali_cache_stamped = '__JABALI_CACHE_PLUGIN_DIR__';
	if ( 0 !== strpos( $jabali_cache_stamped, '__JABALI' ) && is_dir( $jabali_cache_stamped ) ) {
		define( 'JABALI_CACHE_PLUGIN_DIR', $jabali_cache_stamped );
	} elseif ( defined( 'WP_PLUGIN_DIR' ) && is_dir( WP_PLUGIN_DIR . '/jabali-wp-cache' ) ) {
		define( 'JABALI_CACHE_PLUGIN_DIR', WP_PLUGIN_DIR . '/jabali-wp-cache' );
	} else {
		define( 'JABALI_CACHE_PLUGIN_DIR', '' );
	}
}

$jabali_cache_engine = JABALI_CACHE_PLUGIN_DIR . '/includes/class-object-cache.php';
if ( '' !== JABALI_CACHE_PLUGIN_DIR && is_readable( $jabali_cache_engine ) ) {
	require_once JABALI_CACHE_PLUGIN_DIR . '/includes/lib.php';
	require_once $jabali_cache_engine;
}

/**
 * Boot the global cache object. Uses the real engine when available, otherwise
 * a no-op non-persistent shim (mirrors core's default in-memory behaviour).
 */
function wp_cache_init() {
	if ( class_exists( 'Jabali_Cache_Object_Cache' ) ) {
		$GLOBALS['wp_object_cache'] = new Jabali_Cache_Object_Cache();
	} elseif ( ! ( $GLOBALS['wp_object_cache'] ?? null ) instanceof Jabali_Cache_Fallback_Cache ) {
		$GLOBALS['wp_object_cache'] = new Jabali_Cache_Fallback_Cache();
	}
}

/**
 * Minimal non-persistent cache used only when the plugin engine is missing.
 */
class Jabali_Cache_Fallback_Cache {
	private $cache = array();
	public $cache_hits = 0;
	public $cache_misses = 0;
	private $global_groups = array();
	private $non_persistent_groups = array();

	public function add( $key, $data, $group = 'default', $expire = 0 ) {
		if ( isset( $this->cache[ $group ][ $key ] ) ) {
			return false;
		}
		return $this->set( $key, $data, $group, $expire );
	}
	public function set( $key, $data, $group = 'default', $expire = 0 ) {
		$this->cache[ $group ][ $key ] = is_object( $data ) ? clone $data : $data;
		return true;
	}
	public function replace( $key, $data, $group = 'default', $expire = 0 ) {
		if ( ! isset( $this->cache[ $group ][ $key ] ) ) {
			return false;
		}
		return $this->set( $key, $data, $group, $expire );
	}
	public function get( $key, $group = 'default', $force = false, &$found = null ) {
		if ( isset( $this->cache[ $group ] ) && array_key_exists( $key, $this->cache[ $group ] ) ) {
			$found = true;
			$this->cache_hits++;
			$v = $this->cache[ $group ][ $key ];
			return is_object( $v ) ? clone $v : $v;
		}
		$found = false;
		$this->cache_misses++;
		return false;
	}
	public function get_multiple( $keys, $group = 'default', $force = false ) {
		$out = array();
		foreach ( $keys as $k ) {
			$out[ $k ] = $this->get( $k, $group, $force );
		}
		return $out;
	}
	public function set_multiple( array $data, $group = 'default', $expire = 0 ) {
		$out = array();
		foreach ( $data as $k => $v ) {
			$out[ $k ] = $this->set( $k, $v, $group, $expire );
		}
		return $out;
	}
	public function add_multiple( array $data, $group = 'default', $expire = 0 ) {
		$out = array();
		foreach ( $data as $k => $v ) {
			$out[ $k ] = $this->add( $k, $v, $group, $expire );
		}
		return $out;
	}
	public function delete_multiple( array $keys, $group = 'default' ) {
		$out = array();
		foreach ( $keys as $k ) {
			$out[ $k ] = $this->delete( $k, $group );
		}
		return $out;
	}
	public function delete( $key, $group = 'default' ) {
		unset( $this->cache[ $group ][ $key ] );
		return true;
	}
	public function incr( $key, $offset = 1, $group = 'default' ) {
		$cur                          = isset( $this->cache[ $group ][ $key ] ) ? (int) $this->cache[ $group ][ $key ] : 0;
		$cur                          = max( 0, $cur + (int) $offset );
		$this->cache[ $group ][ $key ] = $cur;
		return $cur;
	}
	public function decr( $key, $offset = 1, $group = 'default' ) {
		$cur                          = isset( $this->cache[ $group ][ $key ] ) ? (int) $this->cache[ $group ][ $key ] : 0;
		$cur                          = max( 0, $cur - (int) $offset );
		$this->cache[ $group ][ $key ] = $cur;
		return $cur;
	}
	public function flush() {
		$this->cache = array();
		return true;
	}
	public function flush_group( $group ) {
		unset( $this->cache[ $group ] );
		return true;
	}
	public function flush_runtime() {
		$this->cache = array();
		return true;
	}
	public function supports( $feature ) {
		return in_array( $feature, array( 'add_multiple', 'set_multiple', 'get_multiple', 'delete_multiple', 'flush_runtime', 'flush_group' ), true );
	}
	public function add_global_groups( $groups ) {
		foreach ( (array) $groups as $g ) {
			$this->global_groups[ $g ] = true;
		}
	}
	public function add_non_persistent_groups( $groups ) {
		foreach ( (array) $groups as $g ) {
			$this->non_persistent_groups[ $g ] = true;
		}
	}
	public function switch_to_blog( $blog_id ) {}
	public function close() {}
}

// ---------------------------------------------------------------------------
// wp_cache_* function surface. Thin wrappers over $GLOBALS['wp_object_cache'].
// ---------------------------------------------------------------------------

function wp_cache_add( $key, $data, $group = '', $expire = 0 ) {
	return $GLOBALS['wp_object_cache']->add( $key, $data, $group, (int) $expire );
}
function wp_cache_add_multiple( array $data, $group = '', $expire = 0 ) {
	return $GLOBALS['wp_object_cache']->add_multiple( $data, $group, (int) $expire );
}
function wp_cache_replace( $key, $data, $group = '', $expire = 0 ) {
	return $GLOBALS['wp_object_cache']->replace( $key, $data, $group, (int) $expire );
}
function wp_cache_set( $key, $data, $group = '', $expire = 0 ) {
	return $GLOBALS['wp_object_cache']->set( $key, $data, $group, (int) $expire );
}
function wp_cache_set_multiple( array $data, $group = '', $expire = 0 ) {
	return $GLOBALS['wp_object_cache']->set_multiple( $data, $group, (int) $expire );
}
function wp_cache_get( $key, $group = '', $force = false, &$found = null ) {
	return $GLOBALS['wp_object_cache']->get( $key, $group, $force, $found );
}
function wp_cache_get_multiple( $keys, $group = '', $force = false ) {
	return $GLOBALS['wp_object_cache']->get_multiple( $keys, $group, $force );
}
function wp_cache_delete( $key, $group = '' ) {
	return $GLOBALS['wp_object_cache']->delete( $key, $group );
}
function wp_cache_delete_multiple( array $keys, $group = '' ) {
	return $GLOBALS['wp_object_cache']->delete_multiple( $keys, $group );
}
function wp_cache_incr( $key, $offset = 1, $group = '' ) {
	return $GLOBALS['wp_object_cache']->incr( $key, $offset, $group );
}
function wp_cache_decr( $key, $offset = 1, $group = '' ) {
	return $GLOBALS['wp_object_cache']->decr( $key, $offset, $group );
}
function wp_cache_flush() {
	return $GLOBALS['wp_object_cache']->flush();
}
function wp_cache_flush_runtime() {
	return $GLOBALS['wp_object_cache']->flush_runtime();
}
function wp_cache_flush_group( $group ) {
	return $GLOBALS['wp_object_cache']->flush_group( $group );
}
function wp_cache_supports( $feature ) {
	return $GLOBALS['wp_object_cache']->supports( $feature );
}
function wp_cache_close() {
	return $GLOBALS['wp_object_cache']->close();
}
function wp_cache_add_global_groups( $groups ) {
	$GLOBALS['wp_object_cache']->add_global_groups( $groups );
}
function wp_cache_add_non_persistent_groups( $groups ) {
	$GLOBALS['wp_object_cache']->add_non_persistent_groups( $groups );
}
function wp_cache_switch_to_blog( $blog_id ) {
	$GLOBALS['wp_object_cache']->switch_to_blog( $blog_id );
}
function wp_cache_reset() {
	// Deprecated in core; kept for back-compat. No-op beyond runtime flush.
	if ( method_exists( $GLOBALS['wp_object_cache'], 'flush_runtime' ) ) {
		$GLOBALS['wp_object_cache']->flush_runtime();
	}
}
