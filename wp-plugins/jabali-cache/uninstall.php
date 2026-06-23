<?php
/**
 * Jabali Cache — uninstall cleanup.
 *
 * Runs when the plugin is deleted from the WordPress admin. Removes drop-ins,
 * the generated config file, and the stored settings option. Cached data in
 * Redis is scoped to this site's prefix and expires under LRU; we also make a
 * best-effort prefix-scoped purge here (never FLUSHDB — DB 1 is shared).
 *
 * @package Jabali_Cache
 */

defined( 'WP_UNINSTALL_PLUGIN' ) || exit;

require_once __DIR__ . '/includes/lib.php';
require_once __DIR__ . '/includes/class-settings.php';
require_once __DIR__ . '/includes/class-dropin-manager.php';

// Best-effort: purge this site's keys from Redis.
$jabali_cache_cfg    = Jabali_Cache_Config::load();
$jabali_cache_client = new Jabali_Cache_Client( $jabali_cache_cfg );
if ( $jabali_cache_client->connect() ) {
	$jabali_cache_client->delete_by_pattern( $jabali_cache_cfg['prefix'] . '*' );
	$jabali_cache_client->close();
}

// Remove drop-ins + generated config.
$jabali_cache_mgr = new Jabali_Cache_Dropin_Manager( __DIR__ );
$jabali_cache_mgr->remove();
Jabali_Cache_Settings::delete_config_file();

// Drop settings.
delete_option( Jabali_Cache_Settings::OPTION );
