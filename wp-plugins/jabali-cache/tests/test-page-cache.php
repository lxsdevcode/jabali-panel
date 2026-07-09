<?php
/**
 * Jabali Cache — page-cache request/response gate tests (JAB-60, JAB-61).
 *
 *   php tests/test-page-cache.php
 *
 * Exits non-zero on the first failure. No PHPUnit / WordPress needed: the gates
 * are static + array-injected.
 *
 * @package Jabali_Cache
 */

error_reporting( E_ALL );

// class-page-cache.php guards against direct access with
// `defined( 'ABSPATH' ) || exit;` — satisfy it so we can load the class
// standalone and exercise its static gates.
define( 'ABSPATH', __DIR__ . '/' );

$tests  = 0;
$failed = 0;

function jc_assert( $cond, $msg ) {
	global $tests, $failed;
	$tests++;
	if ( $cond ) {
		echo "  ok   - {$msg}\n";
	} else {
		$failed++;
		echo "  FAIL - {$msg}\n";
	}
}

require __DIR__ . '/../includes/class-page-cache.php';

echo "JAB-60 — auth credentials bypass the page cache:\n";
jc_assert( Jabali_Cache_Page_Cache::has_auth_credentials( array( 'HTTP_AUTHORIZATION' => 'Bearer x' ) ), 'HTTP_AUTHORIZATION bypasses' );
jc_assert( Jabali_Cache_Page_Cache::has_auth_credentials( array( 'REDIRECT_HTTP_AUTHORIZATION' => 'Basic x' ) ), 'REDIRECT_HTTP_AUTHORIZATION bypasses' );
jc_assert( Jabali_Cache_Page_Cache::has_auth_credentials( array( 'PHP_AUTH_USER' => 'bob' ) ), 'PHP_AUTH_USER bypasses' );
jc_assert( Jabali_Cache_Page_Cache::has_auth_credentials( array( 'PHP_AUTH_PW' => 'pw' ) ), 'PHP_AUTH_PW bypasses' );
jc_assert( Jabali_Cache_Page_Cache::has_auth_credentials( array( 'PHP_AUTH_DIGEST' => 'x' ) ), 'PHP_AUTH_DIGEST bypasses' );
jc_assert( ! Jabali_Cache_Page_Cache::has_auth_credentials( array( 'REQUEST_METHOD' => 'GET' ) ), 'anonymous GET does not bypass' );
jc_assert( ! Jabali_Cache_Page_Cache::has_auth_credentials( array( 'HTTP_AUTHORIZATION' => '' ) ), 'empty auth header does not bypass' );

echo "JAB-61 — response cache-control opt-outs:\n";
jc_assert( Jabali_Cache_Page_Cache::response_opts_out( array( 'Cache-Control: private' ) ), 'Cache-Control: private opts out' );
jc_assert( Jabali_Cache_Page_Cache::response_opts_out( array( 'Cache-Control: no-store' ) ), 'no-store opts out' );
jc_assert( Jabali_Cache_Page_Cache::response_opts_out( array( 'cache-control: NO-CACHE' ) ), 'no-cache (case-insensitive) opts out' );
jc_assert( Jabali_Cache_Page_Cache::response_opts_out( array( 'Cache-Control: public, max-age=0' ) ), 'max-age=0 opts out' );
jc_assert( Jabali_Cache_Page_Cache::response_opts_out( array( 'Pragma: no-cache' ) ), 'Pragma: no-cache opts out' );
jc_assert( Jabali_Cache_Page_Cache::response_opts_out( array( 'Vary: Cookie' ) ), 'Vary: Cookie opts out' );
jc_assert( Jabali_Cache_Page_Cache::response_opts_out( array( 'Vary: Authorization' ) ), 'Vary: Authorization opts out' );
jc_assert( Jabali_Cache_Page_Cache::response_opts_out( array( 'Expires: Thu, 01 Jan 1970 00:00:00 GMT' ) ), 'past Expires opts out' );
jc_assert( ! Jabali_Cache_Page_Cache::response_opts_out( array( 'Cache-Control: public, max-age=300' ) ), 'public max-age=300 still cacheable' );
jc_assert( ! Jabali_Cache_Page_Cache::response_opts_out( array( 'Content-Type: text/html' ) ), 'plain response still cacheable' );

echo "\n{$tests} tests, {$failed} failed\n";
exit( $failed > 0 ? 1 : 0 );
