<?php
/**
 * Plugin Name:       Jabali Migrator
 * Plugin URI:        https://jabali-panel.com/
 * Description:       Migrate this WordPress site into a Jabali panel — no SSH. Generates a one-time token; the Jabali panel PULLS the database + files over an authenticated REST API. Pairs with the Jabali panel's WordPress-plugin migration flow (GH #648).
 * Version:           0.1.2
 * Requires at least: 5.6
 * Requires PHP:      7.4
 * Author:            Jabali
 * License:           GPL-2.0-or-later
 * Text Domain:       jabali-migrator
 *
 * SECURITY MODEL (pull): the panel authenticates every request with a Bearer
 * token whose SHA-256 hash is stored in an option (plaintext shown ONCE). All
 * endpoints require it (constant-time compare). DB credentials are NEVER sent to
 * the browser; the DB export runs server-side via mysqldump with a temp
 * defaults-file (password never on the command line). This is a FIRST CUT:
 * resumable chunked transport + a pure-PHP export fallback are follow-ups.
 */

if ( ! defined( 'ABSPATH' ) ) {
	exit;
}

define( 'JABALI_MIGRATOR_VERSION', '0.1.2' );
define( 'JABALI_MIGRATOR_NS', 'jabali-migrator/v1' );
define( 'JABALI_MIGRATOR_TOKEN_OPTION', 'jabali_migrator_token_hash' );

/**
 * Generate a fresh 256-bit token, store ONLY its hash, return the plaintext once.
 */
function jabali_migrator_generate_token() {
	$token = bin2hex( random_bytes( 32 ) );
	update_option( JABALI_MIGRATOR_TOKEN_OPTION, hash( 'sha256', $token ), false );
	return $token;
}

/**
 * Constant-time Bearer-token check — the permission callback for every route.
 */
function jabali_migrator_authorized( WP_REST_Request $request ) {
	$stored = get_option( JABALI_MIGRATOR_TOKEN_OPTION, '' );
	if ( ! is_string( $stored ) || '' === $stored ) {
		return new WP_Error( 'jabali_no_token', 'No migration token configured.', array( 'status' => 403 ) );
	}
	$header = (string) $request->get_header( 'authorization' );
	if ( 0 !== stripos( $header, 'Bearer ' ) ) {
		return new WP_Error( 'jabali_no_bearer', 'Missing Bearer token.', array( 'status' => 401 ) );
	}
	$presented = trim( substr( $header, 7 ) );
	if ( ! hash_equals( $stored, hash( 'sha256', $presented ) ) ) {
		return new WP_Error( 'jabali_bad_token', 'Invalid migration token.', array( 'status' => 401 ) );
	}
	return true;
}

add_action( 'rest_api_init', 'jabali_migrator_register_routes' );
function jabali_migrator_register_routes() {
	$auth = 'jabali_migrator_authorized';

	register_rest_route( JABALI_MIGRATOR_NS, '/ping', array(
		'methods'             => 'GET',
		'permission_callback' => $auth,
		'callback'            => 'jabali_migrator_ping',
	) );

	register_rest_route( JABALI_MIGRATOR_NS, '/manifest', array(
		'methods'             => 'GET',
		'permission_callback' => $auth,
		'callback'            => 'jabali_migrator_manifest',
	) );

	register_rest_route( JABALI_MIGRATOR_NS, '/db-export', array(
		'methods'             => 'GET',
		'permission_callback' => $auth,
		'callback'            => 'jabali_migrator_db_export',
	) );

	register_rest_route( JABALI_MIGRATOR_NS, '/files-manifest', array(
		'methods'             => 'GET',
		'permission_callback' => $auth,
		'callback'            => 'jabali_migrator_files_manifest',
	) );

	register_rest_route( JABALI_MIGRATOR_NS, '/files-archive', array(
		'methods'             => 'GET',
		'permission_callback' => $auth,
		'callback'            => 'jabali_migrator_files_archive',
	) );

	register_rest_route( JABALI_MIGRATOR_NS, '/file', array(
		'methods'             => 'GET',
		'permission_callback' => $auth,
		'callback'            => 'jabali_migrator_file',
		'args'                => array(
			'path' => array( 'required' => true, 'type' => 'string' ),
		),
	) );
}

/** ping — cheap reachability + version probe. */
function jabali_migrator_ping() {
	return array(
		'ok'          => true,
		'plugin'      => 'jabali-migrator',
		'version'     => JABALI_MIGRATOR_VERSION,
		'wp_version'  => get_bloginfo( 'version' ),
		'php_version' => PHP_VERSION,
		'site_url'    => site_url(),
	);
}

/** manifest — everything the panel needs to plan capacity + the import. */
function jabali_migrator_manifest() {
	global $wpdb, $table_prefix;

	// DB size estimate from information_schema (no full scan).
	$db_bytes = 0;
	$rows = $wpdb->get_results(
		$wpdb->prepare(
			'SELECT SUM(data_length + index_length) AS b FROM information_schema.TABLES WHERE table_schema = %s',
			DB_NAME
		)
	);
	if ( ! empty( $rows ) && isset( $rows[0]->b ) ) {
		$db_bytes = (int) $rows[0]->b;
	}

	list( $file_count, $file_bytes ) = jabali_migrator_dir_stats( ABSPATH );

	return array(
		'home'          => get_option( 'home' ),
		'siteurl'       => get_option( 'siteurl' ),
		'table_prefix'  => $table_prefix,
		'wp_version'    => get_bloginfo( 'version' ),
		'php_version'   => PHP_VERSION,
		'multisite'     => is_multisite(),
		'db_bytes'      => $db_bytes,
		'file_count'    => $file_count,
		'file_bytes'    => $file_bytes,
		'wp_root'       => untrailingslashit( ABSPATH ),
		'active_theme'  => (string) get_option( 'stylesheet' ),
		'active_plugins'=> (array) get_option( 'active_plugins', array() ),
	);
}

/**
 * db-export — stream a mysqldump of this site's DB. Credentials come from
 * wp-config (DB_* constants); the password goes into a 0600 temp
 * defaults-extra-file, NEVER onto the command line (argv is world-readable via
 * /proc). The temp file is unlinked immediately after the process starts.
 * FIRST CUT: whole dump in one response (no range chunking yet); requires
 * shell_exec + mysqldump. A pure-PHP $wpdb fallback is a follow-up.
 */
function jabali_migrator_db_export() {
	// Prefer mysqldump (fast), but MANY hosts (including Jabali) disable proc_open
	// in php-fpm for security — fall back to a pure-PHP $wpdb dump so export ALWAYS
	// works, not just where shelling out is allowed.
	$mysqldump = function_exists( 'proc_open' ) ? jabali_migrator_find_bin( 'mysqldump' ) : '';
	if ( '' === $mysqldump ) {
		nocache_headers();
		header( 'Content-Type: application/sql; charset=utf-8' );
		header( 'Content-Disposition: attachment; filename="jabali-migrator-db.sql"' );
		jabali_migrator_php_export();
		exit;
	}

	$cnf = tempnam( sys_get_temp_dir(), 'jabmig' );
	if ( false === $cnf ) {
		return new WP_Error( 'jabali_tmp', 'Could not create temp file.', array( 'status' => 500 ) );
	}
	chmod( $cnf, 0600 );
	// password only in the (0600) file, never in argv.
	file_put_contents( $cnf, "[client]\npassword=" . DB_PASSWORD . "\n" );

	$host = DB_HOST;
	$port = '';
	if ( false !== strpos( $host, ':' ) ) {
		list( $host, $port ) = explode( ':', $host, 2 );
	}
	$args = array(
		$mysqldump,
		'--defaults-extra-file=' . $cnf,
		'--user=' . DB_USER,
		'--host=' . $host,
		'--single-transaction', '--quick', '--skip-lock-tables', '--no-tablespaces',
	);
	if ( '' !== $port && is_numeric( $port ) ) {
		$args[] = '--port=' . $port;
	}
	$args[] = DB_NAME;

	// Stream stdout straight to the client; unlink the cnf once running.
	nocache_headers();
	header( 'Content-Type: application/sql; charset=utf-8' );
	header( 'Content-Disposition: attachment; filename="jabali-migrator-db.sql"' );

	$descriptors = array( 1 => array( 'pipe', 'w' ), 2 => array( 'pipe', 'w' ) );
	$proc = proc_open( jabali_migrator_argv_to_cmd( $args ), $descriptors, $pipes );
	@unlink( $cnf );
	if ( ! is_resource( $proc ) ) {
		return new WP_Error( 'jabali_dump_spawn', 'Could not start mysqldump.', array( 'status' => 500 ) );
	}
	while ( ! feof( $pipes[1] ) ) {
		echo fread( $pipes[1], 65536 );
		flush();
	}
	fclose( $pipes[1] );
	fclose( $pipes[2] );
	proc_close( $proc );
	exit; // raw stream — bypass the REST JSON envelope.
}

/** files-manifest — flat list of files under ABSPATH with size + sha1. */
function jabali_migrator_files_manifest() {
	$root  = untrailingslashit( ABSPATH );
	$out   = array();
	$it    = new RecursiveIteratorIterator(
		new RecursiveDirectoryIterator( $root, FilesystemIterator::SKIP_DOTS ),
		RecursiveIteratorIterator::LEAVES_ONLY
	);
	$skip  = array( '/wp-content/cache/', '/wp-content/object-cache.php', '/wp-content/advanced-cache.php' );
	foreach ( $it as $file ) {
		$abs = $file->getPathname();
		$rel = ltrim( substr( $abs, strlen( $root ) ), '/' );
		$hit = false;
		foreach ( $skip as $s ) {
			if ( false !== strpos( '/' . $rel, $s ) ) { $hit = true; break; }
		}
		if ( $hit || ! $file->isFile() ) {
			continue;
		}
		$out[] = array( 'path' => $rel, 'size' => $file->getSize() );
	}
	return array( 'root' => $root, 'files' => $out );
}

/**
 * file — return one file's bytes. Path is validated to stay UNDER ABSPATH
 * (realpath containment) so the token can never read outside the WP tree.
 */
function jabali_migrator_file( WP_REST_Request $request ) {
	$root = realpath( untrailingslashit( ABSPATH ) );
	$rel  = (string) $request->get_param( 'path' );
	$abs  = realpath( $root . '/' . ltrim( $rel, '/' ) );
	if ( false === $abs || 0 !== strpos( $abs, $root . '/' ) || ! is_file( $abs ) ) {
		return new WP_Error( 'jabali_bad_path', 'Path outside the WordPress root or not a file.', array( 'status' => 400 ) );
	}
	nocache_headers();
	header( 'Content-Type: application/octet-stream' );
	header( 'Content-Length: ' . filesize( $abs ) );
	readfile( $abs );
	exit;
}

/* ------------------------------- helpers -------------------------------- */

function jabali_migrator_dir_stats( $root ) {
	$count = 0; $bytes = 0;
	try {
		$it = new RecursiveIteratorIterator(
			new RecursiveDirectoryIterator( $root, FilesystemIterator::SKIP_DOTS ),
			RecursiveIteratorIterator::LEAVES_ONLY
		);
		foreach ( $it as $f ) {
			if ( $f->isFile() ) { $count++; $bytes += $f->getSize(); }
		}
	} catch ( Exception $e ) {
		// best-effort estimate
	}
	return array( $count, $bytes );
}

function jabali_migrator_find_bin( $name ) {
	foreach ( array( '/usr/bin/', '/usr/local/bin/', '/bin/' ) as $d ) {
		if ( is_executable( $d . $name ) ) { return $d . $name; }
	}
	return '';
}

/** Build a shell command line from an argv array, each arg single-quoted. */
function jabali_migrator_argv_to_cmd( array $argv ) {
	return implode( ' ', array_map( 'escapeshellarg', $argv ) );
}


/**
 * jabali_migrator_php_export — pure-PHP DB dump via $wpdb (no shell). Streams
 * CREATE + chunked INSERTs per table; used when proc_open/mysqldump is
 * unavailable (Jabali + hardened hosts). Values escaped with $wpdb->_real_escape.
 */
function jabali_migrator_php_export() {
	global $wpdb;
	echo "-- jabali-migrator pure-PHP export\n";
	echo "SET NAMES utf8mb4;\nSET FOREIGN_KEY_CHECKS=0;\n";
	$tables = $wpdb->get_col( 'SHOW TABLES' );
	foreach ( $tables as $table ) {
		$create = $wpdb->get_row( "SHOW CREATE TABLE `" . str_replace( '`', '', $table ) . "`", ARRAY_N );
		if ( empty( $create[1] ) ) {
			continue;
		}
		echo "\nDROP TABLE IF EXISTS `" . $table . "`;\n";
		echo $create[1] . ";\n";
		$offset = 0;
		$chunk  = 500;
		do {
			$rows = $wpdb->get_results( "SELECT * FROM `" . str_replace( '`', '', $table ) . "` LIMIT $chunk OFFSET $offset", ARRAY_A );
			foreach ( $rows as $row ) {
				$vals = array();
				foreach ( $row as $v ) {
					$vals[] = is_null( $v ) ? 'NULL' : "'" . $wpdb->_real_escape( $v ) . "'";
				}
				echo "INSERT INTO `" . $table . "` VALUES (" . implode( ',', $vals ) . ");\n";
			}
			$offset += $chunk;
			if ( function_exists( 'ob_get_level' ) && ob_get_level() > 0 ) {
				@ob_flush();
			}
			flush();
		} while ( count( $rows ) === $chunk );
	}
	echo "\nSET FOREIGN_KEY_CHECKS=1;\n";
}


/**
 * files-archive — stream the ENTIRE WordPress tree as ONE gzip tarball in a
 * single request. Replaces the file-by-file /file path (thousands of HTTP
 * round-trips = very slow over the internet). Pure-PHP: a streaming ustar
 * writer (GNU @LongLink for >100-char paths so Go's tar reader recovers them)
 * wrapped in a streaming gzip (deflate_add). No shell, no Phar (phar.readonly
 * blocks Phar writes on many hosts).
 */
function jabali_migrator_files_archive() {
	$root = untrailingslashit( ABSPATH );
	nocache_headers();
	header( 'Content-Type: application/gzip' );
	header( 'Content-Disposition: attachment; filename="files.tar.gz"' );
	while ( ob_get_level() > 0 ) { ob_end_clean(); }

	$def = function_exists( 'deflate_init' ) ? deflate_init( ZLIB_ENCODING_GZIP ) : null;
	$emit = function ( $data ) use ( $def ) {
		if ( '' === $data ) { return; }
		echo $def ? deflate_add( $def, $data, ZLIB_NO_FLUSH ) : $data;
	};

	$skip = array( '/wp-content/cache/', '/wp-content/object-cache.php', '/wp-content/advanced-cache.php' );
	$it   = new RecursiveIteratorIterator(
		new RecursiveDirectoryIterator( $root, FilesystemIterator::SKIP_DOTS ),
		RecursiveIteratorIterator::LEAVES_ONLY
	);
	foreach ( $it as $file ) {
		if ( ! $file->isFile() ) { continue; }
		$abs = $file->getPathname();
		$rel = ltrim( substr( $abs, strlen( $root ) ), '/' );
		$hit = false;
		foreach ( $skip as $sk ) { if ( false !== strpos( '/' . $rel, $sk ) ) { $hit = true; break; } }
		if ( $hit ) { continue; }
		$size = $file->getSize();
		// GNU @LongLink for names > 100 bytes.
		if ( strlen( $rel ) > 100 ) {
			$emit( jabali_migrator_tar_header( '././@LongLink', strlen( $rel ) + 1, 'L' ) );
			$emit( jabali_migrator_tar_pad( $rel . "\0", strlen( $rel ) + 1 ) );
		}
		$emit( jabali_migrator_tar_header( $rel, $size, '0' ) );
		$fh = @fopen( $abs, 'rb' );
		if ( false === $fh ) { $emit( str_repeat( "\0", jabali_migrator_tar_blocks( $size ) ) ); continue; }
		$written = 0;
		while ( ! feof( $fh ) ) {
			$chunk = fread( $fh, 262144 );
			if ( '' === $chunk || false === $chunk ) { break; }
			$emit( $chunk );
			$written += strlen( $chunk );
			flush();
		}
		fclose( $fh );
		// pad to a 512 boundary (and cover any short read).
		$emit( str_repeat( "\0", jabali_migrator_tar_blocks( $size ) - $written ) );
	}
	$emit( str_repeat( "\0", 1024 ) ); // two zero blocks = end of archive
	if ( $def ) { echo deflate_add( $def, '', ZLIB_FINISH ); }
	exit;
}

// jabali_migrator_tar_blocks — bytes rounded up to the next 512 multiple.
function jabali_migrator_tar_blocks( $size ) {
	return $size > 0 ? ( (int) ( ( $size + 511 ) / 512 ) ) * 512 : 0;
}

// jabali_migrator_tar_pad — string padded with NULs to a 512 boundary.
function jabali_migrator_tar_pad( $data, $size ) {
	return $data . str_repeat( "\0", jabali_migrator_tar_blocks( $size ) - strlen( $data ) );
}

// jabali_migrator_tar_header — a 512-byte ustar header for one entry.
function jabali_migrator_tar_header( $name, $size, $type ) {
	$prefix = '';
	$nm     = $name;
	if ( strlen( $nm ) > 100 ) {
		$nm = substr( $nm, 0, 100 ); // real name in the @LongLink; this is a stub
	}
	$h  = str_pad( $nm, 100, "\0" );
	$h .= sprintf( "%07o\0", 0644 );          // mode
	$h .= sprintf( "%07o\0", 0 );             // uid
	$h .= sprintf( "%07o\0", 0 );             // gid
	$h .= sprintf( "%011o\0", $size );        // size
	$h .= sprintf( "%011o\0", 0 );            // mtime
	$h .= str_repeat( ' ', 8 );               // chksum placeholder
	$h .= $type;                              // typeflag
	$h .= str_pad( '', 100, "\0" );           // linkname
	$h .= "ustar\0" . '00';                   // magic + version
	$h .= str_pad( '', 32, "\0" );            // uname
	$h .= str_pad( '', 32, "\0" );            // gname
	$h .= str_pad( '', 8, "\0" );             // devmajor
	$h .= str_pad( '', 8, "\0" );             // devminor
	$h .= str_pad( $prefix, 155, "\0" );      // prefix
	$h .= str_repeat( "\0", 12 );             // pad to 512
	$chk = 0;
	for ( $i = 0; $i < 512; $i++ ) { $chk += ord( $h[ $i ] ); }
	$h = substr_replace( $h, sprintf( "%06o\0 ", $chk ), 148, 8 );
	return $h;
}


/* --------------------------- admin token UI ----------------------------- */

add_action( 'admin_menu', 'jabali_migrator_admin_menu' );
function jabali_migrator_admin_menu() {
	add_management_page(
		'Jabali Migrator', 'Jabali Migrator', 'manage_options',
		'jabali-migrator', 'jabali_migrator_admin_page'
	);
}

function jabali_migrator_admin_page() {
	if ( ! current_user_can( 'manage_options' ) ) { return; }
	$new_token = '';
	if ( isset( $_POST['jabali_migrator_gen'] ) && check_admin_referer( 'jabali_migrator_gen' ) ) {
		$new_token = jabali_migrator_generate_token();
	}
	$base = esc_url( rest_url( JABALI_MIGRATOR_NS ) );
	echo '<div class="wrap"><h1>Jabali Migrator</h1>';
	echo '<p>Generate a token, then paste it (and this site URL) into your Jabali panel\'s WordPress migration screen. The panel will pull this site over the token-authenticated REST API below.</p>';
	echo '<form method="post">';
	wp_nonce_field( 'jabali_migrator_gen' );
	echo '<p><button class="button button-primary" name="jabali_migrator_gen" value="1">Generate migration token</button></p>';
	echo '</form>';
	if ( '' !== $new_token ) {
		echo '<div class="notice notice-warning"><p><strong>Copy this token now — it is shown only once:</strong></p>';
		echo '<p><code style="user-select:all;font-size:14px">' . esc_html( $new_token ) . '</code></p></div>';
	} elseif ( get_option( JABALI_MIGRATOR_TOKEN_OPTION, '' ) ) {
		echo '<p><em>A token is configured (hidden). Generate a new one to rotate; the old one stops working.</em></p>';
	}
	echo '<h2>REST base</h2><p><code>' . $base . '</code></p>';
	echo '</div>';
}

/** On deactivate, drop the token so a stale one can\'t be reused. */
register_deactivation_hook( __FILE__, function () {
	delete_option( JABALI_MIGRATOR_TOKEN_OPTION );
} );
