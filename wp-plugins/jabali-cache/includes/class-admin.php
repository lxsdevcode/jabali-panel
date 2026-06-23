<?php
/**
 * Jabali Cache — admin screen.
 *
 * Settings page under Settings → Jabali Cache: connection status, live Redis
 * health, flush button, drop-in install/repair, and the connection form.
 *
 * @package Jabali_Cache
 */

defined( 'ABSPATH' ) || exit;

class Jabali_Cache_Admin {

	const SLUG  = 'jabali-cache';
	const NONCE = 'jabali_cache_action';

	/** @var string */
	private $plugin_dir;

	/** @var string */
	private $plugin_file;

	public function __construct( $plugin_file ) {
		$this->plugin_file = $plugin_file;
		$this->plugin_dir  = dirname( $plugin_file );
	}

	public function hooks() {
		add_action( 'admin_menu', array( $this, 'menu' ) );
		add_action( 'admin_post_jabali_cache_save', array( $this, 'handle_save' ) );
		add_action( 'admin_post_jabali_cache_flush', array( $this, 'handle_flush' ) );
		add_action( 'admin_post_jabali_cache_dropins', array( $this, 'handle_dropins' ) );
		add_action( 'admin_notices', array( $this, 'notices' ) );
		add_action( 'admin_bar_menu', array( $this, 'admin_bar' ), 100 );
		add_filter(
			'plugin_action_links_' . plugin_basename( $plugin_file ),
			array( $this, 'action_links' )
		);
	}

	public function menu() {
		add_options_page(
			'Jabali Cache',
			'Jabali Cache',
			'manage_options',
			self::SLUG,
			array( $this, 'render' )
		);
	}

	public function action_links( $links ) {
		$url = admin_url( 'options-general.php?page=' . self::SLUG );
		array_unshift( $links, '<a href="' . esc_url( $url ) . '">' . esc_html__( 'Settings', 'jabali-cache' ) . '</a>' );
		return $links;
	}

	public function admin_bar( $bar ) {
		if ( ! current_user_can( 'manage_options' ) ) {
			return;
		}
		$bar->add_node(
			array(
				'id'    => 'jabali-cache-flush',
				'title' => 'Flush Cache',
				'href'  => wp_nonce_url( admin_url( 'admin-post.php?action=jabali_cache_flush' ), self::NONCE ),
				'meta'  => array( 'title' => 'Flush the Jabali object cache' ),
			)
		);
	}

	// ------------------------------------------------------------------
	// Actions.
	// ------------------------------------------------------------------

	public function handle_save() {
		$this->guard();
		$in = array(
			'enabled'    => isset( $_POST['enabled'] ),
			'page_cache' => isset( $_POST['page_cache'] ),
			'page_ttl'   => isset( $_POST['page_ttl'] ) ? (int) $_POST['page_ttl'] : 300,
			'maxttl'     => isset( $_POST['maxttl'] ) ? (int) $_POST['maxttl'] : 0,
			'scheme'     => isset( $_POST['scheme'] ) ? sanitize_text_field( wp_unslash( $_POST['scheme'] ) ) : 'unix',
			'socket'     => isset( $_POST['socket'] ) ? sanitize_text_field( wp_unslash( $_POST['socket'] ) ) : '',
			'host'       => isset( $_POST['host'] ) ? sanitize_text_field( wp_unslash( $_POST['host'] ) ) : '',
			'port'       => isset( $_POST['port'] ) ? (int) $_POST['port'] : 6379,
			'database'   => isset( $_POST['database'] ) ? (int) $_POST['database'] : 1,
			'password'   => isset( $_POST['password'] ) ? sanitize_text_field( wp_unslash( $_POST['password'] ) ) : '',
		);
		Jabali_Cache_Settings::save( $in );
		$this->redirect( 'saved' );
	}

	public function handle_flush() {
		$this->guard();
		$n = 0;
		if ( function_exists( 'wp_cache_flush' ) ) {
			wp_cache_flush();
		}
		// Also purge page cache entries.
		if ( class_exists( 'Jabali_Cache_Page_Cache' ) ) {
			$pc = new Jabali_Cache_Page_Cache();
			$n  = $pc->purge_all();
		}
		$this->redirect( 'flushed', array( 'pages' => $n ) );
	}

	public function handle_dropins() {
		$this->guard();
		$mgr    = new Jabali_Cache_Dropin_Manager( $this->plugin_dir );
		$action = isset( $_POST['dropin_action'] ) ? sanitize_text_field( wp_unslash( $_POST['dropin_action'] ) ) : 'install';
		if ( 'remove' === $action ) {
			$mgr->remove();
			$this->redirect( 'dropins_removed' );
		}
		$res = $mgr->install();
		$this->redirect( ( $res['object'] ? 'dropins_installed' : 'dropins_failed' ) );
	}

	// ------------------------------------------------------------------
	// Notices.
	// ------------------------------------------------------------------

	public function notices() {
		if ( ! current_user_can( 'manage_options' ) ) {
			return;
		}
		$screen = function_exists( 'get_current_screen' ) ? get_current_screen() : null;
		$on_page = $screen && false !== strpos( (string) $screen->id, self::SLUG );

		$health = $this->health();
		if ( ! $health['enabled'] ) {
			return;
		}
		// Surface a connection problem anywhere in admin so it isn't silent.
		if ( ! $health['connected'] ) {
			echo '<div class="notice notice-warning"><p><strong>Jabali Cache:</strong> ';
			echo 'Redis is not reachable, so caching is currently inactive (the site still works, just without acceleration). ';
			echo esc_html( $health['hint'] );
			echo ' <a href="' . esc_url( admin_url( 'options-general.php?page=' . self::SLUG ) ) . '">Open settings</a>.</p></div>';
			return;
		}
		if ( $on_page && ! $health['dropin_ok'] ) {
			echo '<div class="notice notice-warning"><p><strong>Jabali Cache:</strong> ';
			echo 'The object-cache drop-in is not installed yet. Use “Install / repair drop-ins” below to enable persistent caching.</p></div>';
		}
	}

	// ------------------------------------------------------------------
	// Render.
	// ------------------------------------------------------------------

	public function render() {
		if ( ! current_user_can( 'manage_options' ) ) {
			wp_die( esc_html__( 'You do not have permission to view this page.', 'jabali-cache' ) );
		}
		$s      = Jabali_Cache_Settings::get();
		$health = $this->health();
		$mgr    = new Jabali_Cache_Dropin_Manager( $this->plugin_dir );
		$dstat  = $mgr->status();
		$flush  = wp_nonce_url( admin_url( 'admin-post.php?action=jabali_cache_flush' ), self::NONCE );

		echo '<div class="wrap"><h1>Jabali Cache</h1>';
		$this->render_flash();

		// Status card.
		echo '<h2>Status</h2><table class="widefat striped" style="max-width:760px"><tbody>';
		$this->row( 'Caching enabled', $s['enabled'] ? 'Yes' : 'No' );
		$this->row( 'Redis connection', $health['connected'] ? '<span style="color:#1a7f37">Connected</span>' : '<span style="color:#b32d2e">Not reachable</span>' );
		$this->row( 'Driver', $health['connected'] ? esc_html( $health['driver'] ) : '—' );
		$this->row( 'Serializer', esc_html( $health['serializer'] ) );
		$this->row( 'Target', esc_html( $health['target'] ) . ' (db ' . (int) $s['database'] . ')' );
		$this->row( 'Key prefix', '<code>' . esc_html( $health['prefix'] ) . '</code>' );
		$this->row( 'Keys for this site', $health['connected'] ? (int) $health['keys'] : '—' );
		$oc = isset( $GLOBALS['wp_object_cache'] ) ? $GLOBALS['wp_object_cache'] : null;
		if ( $oc && isset( $oc->cache_hits ) ) {
			$h    = (int) $oc->cache_hits;
			$m    = (int) $oc->cache_misses;
			$tot  = $h + $m;
			$rate = $tot ? (int) round( 100 * $h / $tot ) : 0;
			$this->row( 'Cache hits (this request)', esc_html( $h . ' hits / ' . $m . ' misses (' . $rate . '%)' ) );
		}
		$ttl = (int) $s['maxttl'];
		$this->row( 'Object TTL', $ttl > 0 ? esc_html( $ttl . 's' ) : 'none (Redis LRU eviction)' );
		$this->row( 'Object-cache drop-in', $this->dropin_label( $dstat['object_installed'], $dstat['object_ours'], $dstat['object_foreign'] ) );
		if ( empty( $s['page_cache'] ) ) {
			// Page caching is intentionally off (Jabali serves pages from the nginx
			// fastcgi microcache). Show it as a deliberate state, not a red error.
			$this->row( 'Advanced-cache drop-in', '<span style="color:#646970">Off — page caching handled by the server</span>' );
		} else {
			$this->row( 'Advanced-cache drop-in', $this->dropin_label( $dstat['advanced_installed'], $dstat['advanced_ours'], false ) . ( $dstat['wp_cache_const'] ? '' : ' <em>(WP_CACHE not defined in wp-config.php)</em>' ) );
		}
		if ( $health['connected'] && ! empty( $health['server'] ) ) {
			$sv    = $health['server'];
			$parts = array();
			if ( ! empty( $sv['redis_version'] ) ) {
				$parts[] = 'v' . $sv['redis_version'];
			}
			if ( ! empty( $sv['used_memory_human'] ) ) {
				$parts[] = $sv['used_memory_human'] . ' used';
			}
			if ( ! empty( $sv['maxmemory_policy'] ) ) {
				$parts[] = $sv['maxmemory_policy'];
			}
			if ( isset( $sv['evicted_keys'] ) ) {
				$parts[] = (int) $sv['evicted_keys'] . ' evicted';
			}
			$this->row( 'Redis server', esc_html( implode( ' · ', $parts ) ) );
		}
		if ( ! $health['connected'] && '' !== $health['last_error'] ) {
			$this->row( 'Last error', '<code>' . esc_html( $health['last_error'] ) . '</code>' );
		}
		echo '</tbody></table>';

		if ( ! $health['connected'] && $s['enabled'] ) {
			echo '<div class="notice notice-info inline" style="max-width:760px"><p>' . esc_html( $health['hint'] ) . '</p></div>';
		}

		// Quick actions.
		echo '<p style="margin-top:1em">';
		echo '<a href="' . esc_url( $flush ) . '" class="button button-secondary">Flush cache now</a> ';
		echo '</p>';

		// Drop-in management.
		echo '<form method="post" action="' . esc_url( admin_url( 'admin-post.php' ) ) . '" style="margin:1em 0">';
		wp_nonce_field( self::NONCE );
		echo '<input type="hidden" name="action" value="jabali_cache_dropins">';
		echo '<button class="button" name="dropin_action" value="install">Install / repair drop-ins</button> ';
		echo '<button class="button" name="dropin_action" value="remove" onclick="return confirm(\'Remove the Jabali Cache drop-ins?\')">Remove drop-ins</button>';
		echo '</form>';

		// Settings form.
		echo '<h2>Settings</h2>';
		echo '<form method="post" action="' . esc_url( admin_url( 'admin-post.php' ) ) . '">';
		wp_nonce_field( self::NONCE );
		echo '<input type="hidden" name="action" value="jabali_cache_save">';
		echo '<table class="form-table" role="presentation"><tbody>';

		$this->checkbox_row( 'enabled', 'Enable caching', $s['enabled'], 'Master switch for object + page caching.' );
		$this->checkbox_row( 'page_cache', 'Full-page cache', $s['page_cache'], 'Optional. Off by default — the jabali nginx microcache already caches anonymous pages at the edge. Enable only if you disabled that.' );
		$this->number_row( 'page_ttl', 'Page cache TTL (seconds)', $s['page_ttl'] );
		$this->number_row( 'maxttl', 'Object max TTL (seconds, 0 = none)', $s['maxttl'] );

		echo '<tr><th scope="row">Connection</th><td>';
		echo '<label><input type="radio" name="scheme" value="unix" ' . checked( $s['scheme'], 'unix', false ) . '> Unix socket</label> &nbsp; ';
		echo '<label><input type="radio" name="scheme" value="tcp" ' . checked( $s['scheme'], 'tcp', false ) . '> TCP</label>';
		echo '</td></tr>';
		$this->text_row( 'socket', 'Socket path', $s['socket'], '/run/redis/redis.sock' );
		$this->text_row( 'host', 'TCP host', $s['host'], '127.0.0.1' );
		$this->number_row( 'port', 'TCP port', $s['port'] );
		$this->number_row( 'database', 'Redis database', $s['database'] );
		$this->password_row( 'password', 'Password (optional)', $s['password'] );

		echo '</tbody></table>';
		submit_button( 'Save settings' );
		echo '</form>';

		echo '<hr><p style="color:#646970;max-width:760px">Jabali Cache uses the shared panel Redis (ADR-0059): unix socket <code>/run/redis/redis.sock</code>, database 1. Cache entries are isolated per site by key prefix and survive Redis LRU eviction by design.</p>';
		echo '</div>';
	}

	// ------------------------------------------------------------------
	// Helpers.
	// ------------------------------------------------------------------

	/**
	 * @return array<string,mixed>
	 */
	private function health() {
		$s   = Jabali_Cache_Settings::get();
		$cfg = Jabali_Cache_Config::load();
		$out = array(
			'enabled'    => (bool) $s['enabled'],
			'connected'  => false,
			'driver'     => '',
			'serializer' => function_exists( 'igbinary_serialize' ) ? 'igbinary' : 'php',
			'prefix'     => $cfg['prefix'],
			'target'     => ( 'unix' === $s['scheme'] ) ? $s['socket'] : ( $s['host'] . ':' . $s['port'] ),
			'keys'       => 0,
			'last_error' => '',
			'dropin_ok'  => false,
			'hint'       => '',
			'server'     => array(),
		);

		$mgr              = new Jabali_Cache_Dropin_Manager( $this->plugin_dir );
		$dstat            = $mgr->status();
		$out['dropin_ok'] = $dstat['object_ours'];

		$client = new Jabali_Cache_Client( $cfg );
		if ( $client->connect() ) {
			$out['connected']  = true;
			$out['driver']     = $client->driver();
			$out['keys']       = $client->count_keys( $cfg['prefix'] );
			$out['server']     = $this->parse_info( $client->info() );
			$client->close();
		} else {
			$out['last_error'] = $client->last_error();
			$out['hint']       = $this->diagnose_hint( $cfg, $out['last_error'] );
		}
		return $out;
	}

	/**
	 * Pull a few operationally-useful fields out of a Redis INFO dump.
	 *
	 * @param string $raw
	 * @return array<string,string>
	 */
	private function parse_info( $raw ) {
		$want = array( 'redis_version', 'used_memory_human', 'maxmemory_policy', 'evicted_keys', 'connected_clients' );
		$map  = array();
		foreach ( preg_split( '/\r?\n/', (string) $raw ) as $line ) {
			$kv = explode( ':', $line, 2 );
			if ( 2 === count( $kv ) && in_array( $kv[0], $want, true ) ) {
				$map[ $kv[0] ] = trim( $kv[1] );
			}
		}
		return $map;
	}

	/**
	 * Translate a low-level connection failure into an operator-actionable hint
	 * specific to the jabali shared-hosting environment.
	 */
	private function diagnose_hint( array $cfg, $err ) {
		$err = strtolower( (string) $err );
		if ( 'unix' === $cfg['scheme'] ) {
			if ( false !== strpos( $err, 'open_basedir' ) || false !== strpos( $err, 'operation not permitted' ) ) {
				return 'PHP open_basedir is blocking the Redis socket. Add "/run/redis/redis.sock" (or "/run/redis") to this site\'s open_basedir, or ask the panel admin to allow it for the PHP-FPM pool.';
			}
			if ( false !== strpos( $err, 'permission denied' ) ) {
				return 'The PHP-FPM user cannot open the Redis socket. On the Jabali panel this is provisioned automatically when you enable caching for the site (Applications → cache toggle) — re-run that if you see this. On a standalone host, grant the site\'s PHP user read/write on the socket.';
			}
			if ( false !== strpos( $err, 'no such file' ) || false !== strpos( $err, 'connection refused' ) ) {
				return 'Redis socket /run/redis/redis.sock not found. Confirm redis-server is running on the panel host.';
			}
			return 'Could not connect to the Redis unix socket. Verify open_basedir includes /run/redis and that the site\'s PHP user can read the socket (the Jabali panel grants this automatically when caching is enabled).';
		}
		return 'Could not connect to Redis over TCP. Note: the jabali panel runs Redis on a unix socket only (no TCP) — prefer the Unix socket option.';
	}

	private function dropin_label( $installed, $ours, $foreign ) {
		if ( ! $installed ) {
			return '<span style="color:#b32d2e">Not installed</span>';
		}
		if ( $foreign ) {
			return '<span style="color:#b32d2e">A different object-cache.php is present (not ours)</span>';
		}
		return $ours ? '<span style="color:#1a7f37">Installed</span>' : 'Present';
	}

	private function guard() {
		if ( ! current_user_can( 'manage_options' ) ) {
			wp_die( esc_html__( 'Permission denied.', 'jabali-cache' ) );
		}
		check_admin_referer( self::NONCE );
	}

	private function redirect( $flash, array $extra = array() ) {
		$args = array_merge( array( 'page' => self::SLUG, 'jc_flash' => $flash ), $extra );
		wp_safe_redirect( add_query_arg( $args, admin_url( 'options-general.php' ) ) );
		exit;
	}

	private function render_flash() {
		if ( empty( $_GET['jc_flash'] ) ) { // phpcs:ignore WordPress.Security.NonceVerification.Recommended
			return;
		}
		$flash = sanitize_text_field( wp_unslash( $_GET['jc_flash'] ) ); // phpcs:ignore WordPress.Security.NonceVerification.Recommended
		$map   = array(
			'saved'             => array( 'success', 'Settings saved.' ),
			'flushed'           => array( 'success', 'Cache flushed.' ),
			'dropins_installed' => array( 'success', 'Drop-ins installed.' ),
			'dropins_removed'   => array( 'success', 'Drop-ins removed.' ),
			'dropins_failed'    => array( 'error', 'Could not install drop-ins (a foreign object-cache.php may be present, or wp-content is not writable).' ),
		);
		if ( isset( $map[ $flash ] ) ) {
			printf( '<div class="notice notice-%s is-dismissible"><p>%s</p></div>', esc_attr( $map[ $flash ][0] ), esc_html( $map[ $flash ][1] ) );
		}
	}

	private function row( $label, $value ) {
		echo '<tr><td style="width:220px"><strong>' . esc_html( $label ) . '</strong></td><td>' . $value . '</td></tr>'; // phpcs:ignore WordPress.Security.EscapeOutput.OutputNotEscaped
	}

	private function checkbox_row( $name, $label, $checked, $desc = '' ) {
		echo '<tr><th scope="row">' . esc_html( $label ) . '</th><td><label><input type="checkbox" name="' . esc_attr( $name ) . '" ' . checked( $checked, true, false ) . '> ' . esc_html( $desc ) . '</label></td></tr>';
	}

	private function text_row( $name, $label, $value, $ph = '' ) {
		echo '<tr><th scope="row">' . esc_html( $label ) . '</th><td><input type="text" class="regular-text" name="' . esc_attr( $name ) . '" value="' . esc_attr( $value ) . '" placeholder="' . esc_attr( $ph ) . '"></td></tr>';
	}

	private function password_row( $name, $label, $value ) {
		echo '<tr><th scope="row">' . esc_html( $label ) . '</th><td><input type="password" class="regular-text" name="' . esc_attr( $name ) . '" value="' . esc_attr( $value ) . '" autocomplete="new-password"></td></tr>';
	}

	private function number_row( $name, $label, $value ) {
		echo '<tr><th scope="row">' . esc_html( $label ) . '</th><td><input type="number" name="' . esc_attr( $name ) . '" value="' . esc_attr( (int) $value ) . '" class="small-text"></td></tr>';
	}
}
