<?php
/**
 * Jabali Cache — MULTISITE object-cache contract test (GH #626).
 *
 * Verifies the network-namespacing contract the readme claims: on a multisite
 * network, NON-global groups are namespaced per blog (blog A and blog B never
 * see each other's entries), while GLOBAL groups (users, useremail, …) are
 * shared across the whole network. Runs in-memory (no PHPUnit, no WordPress,
 * no Redis) — the in-memory layer keys on the same composed key() the Redis
 * layer does, so the contract holds either way.
 *
 *   php tests/test-object-cache-multisite.php
 *
 * @package Jabali_Cache
 */

error_reporting( E_ALL );

$tests  = 0;
$failed = 0;
function ms_assert( $cond, $msg ) {
	global $tests, $failed;
	$tests++;
	echo ( $cond ? "  ok   - " : "  FAIL - " ) . $msg . "\n";
	if ( ! $cond ) {
		$failed++;
	}
}

// Multisite stubs — is_multisite() TRUE so the engine namespaces per blog.
if ( ! function_exists( 'wp_suspend_cache_addition' ) ) {
	function wp_suspend_cache_addition( $suspend = null ) {
		return false; }
}
if ( ! function_exists( 'is_multisite' ) ) {
	function is_multisite() {
		return true; }
}
if ( ! function_exists( 'get_current_blog_id' ) ) {
	function get_current_blog_id() {
		return 1; }
}

// lib.php + drop-ins guard on ABSPATH (wp.org direct-access guard); define it so
// the engine loads standalone in the test harness.
if ( ! defined( 'ABSPATH' ) ) {
	define( 'ABSPATH', sys_get_temp_dir() . '/' );
}
require __DIR__ . '/../includes/lib.php';

$cfg             = Jabali_Cache_Config::load();
$cfg['socket']   = '/nonexistent/jc.sock'; // in-memory mode.
$cfg['database'] = 1;
$cfg['timeout']  = 1.0;
$cfg['prefix']   = 'jc:mstest' . getmypid() . ':';
Jabali_Cache_Config::set( $cfg );

require __DIR__ . '/../includes/class-object-cache.php';

echo "Multisite object-cache contract (in-memory):\n";

$c = new Jabali_Cache_Object_Cache();

// 'posts' is NOT a global group — must be per-blog namespaced.
$c->switch_to_blog( 1 );
ms_assert( true === $c->set( 'k', 'blog1', 'posts' ), 'blog 1: set non-global key' );
ms_assert( 'blog1' === $c->get( 'k', 'posts' ), 'blog 1: reads its own value' );

$c->switch_to_blog( 2 );
ms_assert( false === $c->get( 'k', 'posts', false, $found ) && false === $found,
	'blog 2: does NOT see blog 1 non-global entry (namespaced)' );
$c->set( 'k', 'blog2', 'posts' );
ms_assert( 'blog2' === $c->get( 'k', 'posts' ), 'blog 2: has its own value' );

$c->switch_to_blog( 1 );
ms_assert( 'blog1' === $c->get( 'k', 'posts' ),
	'blog 1: still its own value — blog 2 did not clobber it' );

// 'users' IS a global group (lib.php defaults) — must be shared across blogs.
$c->switch_to_blog( 1 );
ms_assert( true === $c->set( 'u', 'shared', 'users' ), 'blog 1: set global-group key' );
$c->switch_to_blog( 2 );
ms_assert( 'shared' === $c->get( 'u', 'users' ),
	'blog 2: SEES the global-group entry set on blog 1 (shared)' );

echo "\n{$tests} tests, {$failed} failed\n";
exit( $failed ? 1 : 0 );
