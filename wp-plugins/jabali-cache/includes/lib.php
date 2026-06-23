<?php

defined( 'ABSPATH' ) || exit; // wp.org: prevent direct file access.
/**
 * Jabali Cache — shared low-level library.
 *
 * Loaded by BOTH the plugin and the wp-content drop-ins (object-cache.php,
 * advanced-cache.php). Must be self-sufficient: no WordPress functions are
 * required to load this file, because advanced-cache.php runs before WP is
 * bootstrapped.
 *
 * Contents:
 *   - Jabali_Cache_Config : connection + behaviour configuration resolver.
 *   - Jabali_Cache_Client : Redis client. Uses the phpredis extension when
 *                           present, otherwise a dependency-free pure-PHP
 *                           RESP client over a stream socket.
 *
 * Design constraints (jabali panel, ADR-0059):
 *   - Redis is a SHARED instance reachable at unix:///run/redis/redis.sock.
 *   - Logical DB 1 is reserved for WordPress object cache.
 *   - The instance runs maxmemory-policy allkeys-lru: any key can vanish at
 *     any time, so every read is treated as best-effort.
 *   - DB 1 is shared across all tenants on the host. Isolation is by key
 *     PREFIX only. We therefore NEVER issue FLUSHDB (would wipe other
 *     tenants); flushing is always scoped to our own prefix via SCAN + DEL.
 *
 * @package Jabali_Cache
 */

if ( defined( 'JABALI_CACHE_LIB_LOADED' ) ) {
	return;
}
define( 'JABALI_CACHE_LIB_LOADED', true );

if ( ! defined( 'JABALI_CACHE_VERSION' ) ) {
	define( 'JABALI_CACHE_VERSION', '1.0.0' );
}

/**
 * Resolves runtime configuration from (in priority order):
 *   1. PHP constants (typically set in wp-config.php),
 *   2. the generated config file wp-content/jabali-cache-config.php,
 *   3. built-in jabali defaults.
 *
 * Resolution is done without touching the database so the drop-ins, which
 * load before WordPress is available, can configure themselves.
 */
class Jabali_Cache_Config {

	/** @var array<string,mixed>|null */
	private static $cache = null;

	/**
	 * @return array<string,mixed>
	 */
	public static function load() {
		if ( null !== self::$cache ) {
			return self::$cache;
		}

		$file_cfg = array();
		$path     = self::config_file_path();
		if ( $path && is_readable( $path ) ) {
			/** @psalm-suppress UnresolvableInclude */
			$loaded = include $path;
			if ( is_array( $loaded ) ) {
				$file_cfg = $loaded;
			}
		}

		$defaults = array(
			// Connection. Socket is preferred and is the jabali default.
			'scheme'   => 'unix',                     // 'unix' or 'tcp'.
			'socket'   => '/run/redis/redis.sock',
			'host'     => '127.0.0.1',
			'port'     => 6379,
			'database' => 1,                          // ADR-0059: DB 1 for WP.
			'password' => '',                         // jabali socket has no AUTH.
			'username' => '',                         // Redis ACL user (AUTH <user> <pass>); empty = legacy AUTH.
			'timeout'  => 1.0,                        // connect/read timeout (seconds).

			// Behaviour.
			'enabled'        => true,
			'prefix'         => '',                   // resolved below if empty.
			'maxttl'         => 0,                    // object-cache key TTL; 0 = rely on LRU.
			'page_cache'     => false,                // off by default (nginx microcache exists).
			'page_ttl'       => 300,
			'serializer'     => 'auto',               // auto|igbinary|php.
			'ignored_groups' => array( 'counts', 'plugins', 'themes' ),
			'global_groups'  => array(
				'blog-details', 'blog-id-cache', 'blog-lookup', 'global-posts',
				'networks', 'rss', 'sites', 'site-details', 'site-lookup',
				'site-options', 'site-transient', 'users', 'useremail',
				'userlogins', 'usermeta', 'user_meta', 'userslugs',
			),
			'page_exclusions' => array( '/wp-admin/', '/wp-json/', '/feed/', '/sitemap' ),
		);

		$cfg = self::merge( $defaults, $file_cfg );
		$cfg = self::apply_constants( $cfg );

		if ( '' === $cfg['prefix'] ) {
			$cfg['prefix'] = self::derive_prefix();
		}

		// Normalise the prefix: keep it short, deterministic and key-safe.
		$cfg['prefix'] = 'jc:' . preg_replace( '/[^A-Za-z0-9_.:-]/', '', (string) $cfg['prefix'] ) . ':';

		self::$cache = $cfg;
		return $cfg;
	}

	/**
	 * Allow explicit override (used by the admin "test connection" flow).
	 *
	 * @param array<string,mixed> $cfg
	 */
	public static function set( array $cfg ) {
		self::$cache = $cfg;
	}

	public static function reset() {
		self::$cache = null;
	}

	/**
	 * @return string
	 */
	public static function config_file_path() {
		if ( defined( 'JABALI_CACHE_CONFIG_FILE' ) ) {
			return (string) JABALI_CACHE_CONFIG_FILE;
		}
		if ( defined( 'WP_CONTENT_DIR' ) ) {
			return WP_CONTENT_DIR . '/jabali-cache-config.php';
		}
		// Drop-ins run before WP_CONTENT_DIR may be defined; derive from this file.
		return dirname( __DIR__, 3 ) . '/jabali-cache-config.php';
	}

	/**
	 * @param array<string,mixed> $cfg
	 * @return array<string,mixed>
	 */
	private static function apply_constants( array $cfg ) {
		$map = array(
			'JABALI_CACHE_DISABLED' => array( 'enabled', true ),  // inverted below.
			'JABALI_CACHE_SOCKET'   => array( 'socket', false ),
			'WP_REDIS_PATH'         => array( 'socket', false ),
			'JABALI_CACHE_HOST'     => array( 'host', false ),
			'JABALI_CACHE_PORT'     => array( 'port', false ),
			'JABALI_CACHE_DB'       => array( 'database', false ),
			'WP_REDIS_DATABASE'     => array( 'database', false ),
			'JABALI_CACHE_PASSWORD' => array( 'password', false ),
			'JABALI_CACHE_USER'     => array( 'username', false ),
			'JABALI_CACHE_PREFIX'   => array( 'prefix', false ),
			'JABALI_CACHE_MAXTTL'   => array( 'maxttl', false ),
			'JABALI_CACHE_SCHEME'   => array( 'scheme', false ),
		);
		foreach ( $map as $const => $spec ) {
			if ( ! defined( $const ) ) {
				continue;
			}
			list( $key, $invert ) = $spec;
			$val = constant( $const );
			if ( $invert ) {
				$cfg[ $key ] = ! $val;
			} else {
				$cfg[ $key ] = $val;
			}
		}
		// If a TCP host is explicitly set without a scheme override, switch to tcp.
		if ( defined( 'JABALI_CACHE_HOST' ) && ! defined( 'JABALI_CACHE_SCHEME' ) ) {
			$cfg['scheme'] = 'tcp';
		}
		return $cfg;
	}

	/**
	 * Per-site key prefix. Stable for the lifetime of an install, unique per
	 * site URL + filesystem location so two WP installs on the same tenant
	 * (and on the shared DB) never collide.
	 *
	 * @return string
	 */
	private static function derive_prefix() {
		$seed = '';
		if ( defined( 'ABSPATH' ) ) {
			$seed .= ABSPATH;
		}
		if ( defined( 'DB_NAME' ) ) {
			$seed .= '|' . DB_NAME;
		}
		if ( defined( 'WP_HOME' ) ) {
			$seed .= '|' . WP_HOME;
		} elseif ( isset( $_SERVER['HTTP_HOST'] ) ) {
			$seed .= '|' . $_SERVER['HTTP_HOST']; // phpcs:ignore
		}
		if ( '' === $seed ) {
			$seed = __FILE__;
		}
		return substr( md5( $seed ), 0, 12 );
	}

	/**
	 * Shallow merge that preserves array-typed defaults when override omits them.
	 *
	 * @param array<string,mixed> $base
	 * @param array<string,mixed> $over
	 * @return array<string,mixed>
	 */
	private static function merge( array $base, array $over ) {
		foreach ( $over as $k => $v ) {
			$base[ $k ] = $v;
		}
		return $base;
	}
}

/**
 * Redis client used by the object cache and page cache. Wraps the phpredis
 * extension when available and falls back to a self-contained pure-PHP RESP
 * implementation otherwise (jabali does not install php-redis by default).
 *
 * All methods are failure-tolerant: a connection or protocol error flips the
 * client into a permanently-disconnected state for the remainder of the
 * request and every operation returns a benign miss. The caller (object cache)
 * then behaves as a non-persistent cache — the site keeps working.
 */
class Jabali_Cache_Client {

	/** @var array<string,mixed> */
	private $cfg;

	/** @var \Redis|null phpredis instance. */
	private $redis = null;

	/** @var resource|null raw stream for the pure-PHP path. */
	private $stream = null;

	/** @var bool */
	private $connected = false;

	/** @var bool true once a fatal error disables the client for this request. */
	private $dead = false;

	/** @var string */
	private $last_error = '';

	/** @var string 'phpredis' | 'pure-php' | '' */
	private $driver = '';

	/**
	 * @param array<string,mixed>|null $cfg
	 */
	public function __construct( $cfg = null ) {
		$this->cfg = is_array( $cfg ) ? $cfg : Jabali_Cache_Config::load();
	}

	public function driver() {
		return $this->driver;
	}

	public function last_error() {
		return $this->last_error;
	}

	public function is_connected() {
		return $this->connected && ! $this->dead;
	}

	/**
	 * @return bool
	 */
	public function connect() {
		if ( $this->connected ) {
			return true;
		}
		if ( $this->dead ) {
			return false;
		}
		if ( empty( $this->cfg['enabled'] ) ) {
			$this->fail( 'cache disabled by configuration' );
			return false;
		}

		if ( class_exists( 'Redis' ) ) {
			if ( $this->connect_phpredis() ) {
				return true;
			}
			// Fall through to pure-PHP only if phpredis is present but failed
			// for a non-protocol reason — usually it just means unavailable,
			// in which case pure-PHP will fail too. Try anyway; it is cheap.
		}
		return $this->connect_pure();
	}

	/**
	 * @return bool
	 */
	private function connect_phpredis() {
		try {
			$redis   = new \Redis();
			$timeout = (float) $this->cfg['timeout'];
			if ( 'unix' === $this->cfg['scheme'] ) {
				$ok = $redis->connect( $this->cfg['socket'], 0, $timeout );
			} else {
				$ok = $redis->connect( $this->cfg['host'], (int) $this->cfg['port'], $timeout );
			}
			if ( ! $ok ) {
				return false;
			}
			if ( ! empty( $this->cfg['password'] ) ) {
				// Redis 6+ ACL needs AUTH <user> <pass>; fall back to legacy single-arg.
				if ( ! empty( $this->cfg['username'] ) ) {
					$redis->auth( array( $this->cfg['username'], $this->cfg['password'] ) );
				} else {
					$redis->auth( $this->cfg['password'] );
				}
			}
			$redis->select( (int) $this->cfg['database'] );
			$this->redis     = $redis;
			$this->driver    = 'phpredis';
			$this->connected = true;
			return true;
		} catch ( \Throwable $e ) {
			$this->last_error = 'phpredis: ' . $e->getMessage();
			return false;
		}
	}

	/**
	 * @return bool
	 */
	private function connect_pure() {
		$timeout = (float) $this->cfg['timeout'];
		if ( 'unix' === $this->cfg['scheme'] ) {
			$target = 'unix://' . $this->cfg['socket'];
		} else {
			$target = 'tcp://' . $this->cfg['host'] . ':' . (int) $this->cfg['port'];
		}
		$errno  = 0;
		$errstr = '';
		// @ to keep open_basedir / connection warnings out of the page; the
		// boolean result and graceful-disable path handle the failure.
		$stream = @stream_socket_client( $target, $errno, $errstr, $timeout ); // phpcs:ignore
		if ( ! $stream ) {
			$this->fail( sprintf( 'connect %s failed: %s (%d)', $target, $errstr, $errno ) );
			return false;
		}
		stream_set_timeout( $stream, (int) $timeout, (int) ( ( $timeout - (int) $timeout ) * 1e6 ) );
		$this->stream = $stream;
		$this->driver = 'pure-php';

		if ( ! empty( $this->cfg['password'] ) ) {
			$jabali_auth = ! empty( $this->cfg['username'] )
				? array( 'AUTH', $this->cfg['username'], $this->cfg['password'] )
				: array( 'AUTH', $this->cfg['password'] );
			// Redis replies +OK to AUTH, which read_reply() returns as the string
			// "OK" (not bool true). A wrong cred yields -WRONGPASS -> null.
			if ( 'OK' !== $this->cmd( $jabali_auth ) ) {
				$this->fail( 'AUTH rejected' );
				return false;
			}
		}
		if ( null === $this->cmd( array( 'SELECT', (string) (int) $this->cfg['database'] ) ) && $this->dead ) {
			return false;
		}
		$pong = $this->cmd( array( 'PING' ) );
		if ( 'PONG' !== $pong && true !== $pong ) {
			$this->fail( 'PING did not return PONG' );
			return false;
		}
		$this->connected = true;
		return true;
	}

	private function fail( $msg ) {
		$this->last_error = $msg;
		$this->dead       = true;
		$this->connected  = false;
		if ( is_resource( $this->stream ) ) {
			@fclose( $this->stream ); // phpcs:ignore
		}
		$this->stream = null;
		$this->redis  = null;
	}

	/**
	 * GET. Returns the raw string value or false on miss/error.
	 *
	 * @param string $key
	 * @return string|false
	 */
	public function get( $key ) {
		if ( ! $this->is_connected() ) {
			return false;
		}
		if ( 'phpredis' === $this->driver ) {
			try {
				return $this->redis->get( $key );
			} catch ( \Throwable $e ) {
				$this->fail( $e->getMessage() );
				return false;
			}
		}
		$res = $this->cmd( array( 'GET', $key ) );
		return ( null === $res ) ? false : $res;
	}

	/**
	 * MGET. Returns a map key => value|false (false = miss).
	 *
	 * @param string[] $keys
	 * @return array<string,string|false>
	 */
	public function mget( array $keys ) {
		$out = array();
		if ( empty( $keys ) || ! $this->is_connected() ) {
			foreach ( $keys as $k ) {
				$out[ $k ] = false;
			}
			return $out;
		}
		if ( 'phpredis' === $this->driver ) {
			try {
				$vals = $this->redis->mget( array_values( $keys ) );
			} catch ( \Throwable $e ) {
				$this->fail( $e->getMessage() );
				$vals = array();
			}
			$i = 0;
			foreach ( $keys as $k ) {
				$out[ $k ] = isset( $vals[ $i ] ) ? $vals[ $i ] : false;
				$i++;
			}
			return $out;
		}
		$args = array( 'MGET' );
		foreach ( $keys as $k ) {
			$args[] = $k;
		}
		$vals = $this->cmd( $args );
		$i    = 0;
		foreach ( $keys as $k ) {
			$v         = ( is_array( $vals ) && array_key_exists( $i, $vals ) && null !== $vals[ $i ] ) ? $vals[ $i ] : false;
			$out[ $k ] = $v;
			$i++;
		}
		return $out;
	}

	/**
	 * SET with optional TTL (seconds). Returns bool.
	 *
	 * @param string $key
	 * @param string $value
	 * @param int    $ttl
	 * @return bool
	 */
	public function set( $key, $value, $ttl = 0 ) {
		if ( ! $this->is_connected() ) {
			return false;
		}
		if ( 'phpredis' === $this->driver ) {
			try {
				if ( $ttl > 0 ) {
					return (bool) $this->redis->setex( $key, $ttl, $value );
				}
				return (bool) $this->redis->set( $key, $value );
			} catch ( \Throwable $e ) {
				$this->fail( $e->getMessage() );
				return false;
			}
		}
		if ( $ttl > 0 ) {
			$res = $this->cmd( array( 'SETEX', $key, (string) $ttl, $value ) );
		} else {
			$res = $this->cmd( array( 'SET', $key, $value ) );
		}
		return ( true === $res || 'OK' === $res );
	}

	/**
	 * Conditional SET (NX) — used by add(). Returns true only if stored.
	 *
	 * @param string $key
	 * @param string $value
	 * @param int    $ttl
	 * @return bool
	 */
	public function add( $key, $value, $ttl = 0 ) {
		if ( ! $this->is_connected() ) {
			return false;
		}
		if ( 'phpredis' === $this->driver ) {
			try {
				$opts = array( 'nx' );
				if ( $ttl > 0 ) {
					$opts = array( 'nx', 'ex' => $ttl );
				}
				return (bool) $this->redis->set( $key, $value, $opts );
			} catch ( \Throwable $e ) {
				$this->fail( $e->getMessage() );
				return false;
			}
		}
		$args = array( 'SET', $key, $value, 'NX' );
		if ( $ttl > 0 ) {
			$args[] = 'EX';
			$args[] = (string) $ttl;
		}
		$res = $this->cmd( $args );
		return ( true === $res || 'OK' === $res );
	}

	/**
	 * @param string|string[] $keys
	 * @return int number of keys deleted.
	 */
	public function del( $keys ) {
		if ( ! $this->is_connected() ) {
			return 0;
		}
		$keys = is_array( $keys ) ? array_values( $keys ) : array( $keys );
		if ( empty( $keys ) ) {
			return 0;
		}
		if ( 'phpredis' === $this->driver ) {
			try {
				return (int) $this->redis->del( $keys );
			} catch ( \Throwable $e ) {
				$this->fail( $e->getMessage() );
				return 0;
			}
		}
		$args = array_merge( array( 'DEL' ), $keys );
		$res  = $this->cmd( $args );
		return is_int( $res ) ? $res : 0;
	}

	/**
	 * @param string $key
	 * @param int    $by
	 * @return int|false new value or false.
	 */
	public function incr( $key, $by = 1 ) {
		if ( ! $this->is_connected() ) {
			return false;
		}
		if ( 'phpredis' === $this->driver ) {
			try {
				return $this->redis->incrBy( $key, $by );
			} catch ( \Throwable $e ) {
				$this->fail( $e->getMessage() );
				return false;
			}
		}
		$res = $this->cmd( array( 'INCRBY', $key, (string) $by ) );
		return is_int( $res ) ? $res : false;
	}

	/**
	 * @param string $key
	 * @param int    $by
	 * @return int|false
	 */
	public function decr( $key, $by = 1 ) {
		if ( ! $this->is_connected() ) {
			return false;
		}
		if ( 'phpredis' === $this->driver ) {
			try {
				return $this->redis->decrBy( $key, $by );
			} catch ( \Throwable $e ) {
				$this->fail( $e->getMessage() );
				return false;
			}
		}
		$res = $this->cmd( array( 'DECRBY', $key, (string) $by ) );
		return is_int( $res ) ? $res : false;
	}

	/**
	 * Delete every key matching a glob-style pattern, using SCAN to avoid a
	 * blocking KEYS sweep. NEVER use FLUSHDB — DB 1 is shared across tenants.
	 *
	 * @param string $pattern
	 * @return int number of keys deleted.
	 */
	public function delete_by_pattern( $pattern ) {
		if ( ! $this->is_connected() ) {
			return 0;
		}
		$deleted = 0;
		if ( 'phpredis' === $this->driver ) {
			try {
				$it = null;
				$this->redis->setOption( \Redis::OPT_SCAN, \Redis::SCAN_RETRY );
				while ( false !== ( $keys = $this->redis->scan( $it, $pattern, 500 ) ) ) {
					if ( ! empty( $keys ) ) {
						$deleted += (int) $this->redis->del( $keys );
					}
					if ( 0 === (int) $it ) {
						break;
					}
				}
			} catch ( \Throwable $e ) {
				$this->fail( $e->getMessage() );
			}
			return $deleted;
		}
		$cursor = '0';
		do {
			$res = $this->cmd( array( 'SCAN', $cursor, 'MATCH', $pattern, 'COUNT', '500' ) );
			if ( ! is_array( $res ) || count( $res ) < 2 ) {
				break;
			}
			$cursor = (string) $res[0];
			$batch  = is_array( $res[1] ) ? $res[1] : array();
			if ( ! empty( $batch ) ) {
				$deleted += $this->del( $batch );
			}
		} while ( '0' !== $cursor && ! $this->dead );
		return $deleted;
	}

	/**
	 * @return string raw INFO output (or '' on failure).
	 */
	public function info() {
		if ( ! $this->is_connected() ) {
			return '';
		}
		if ( 'phpredis' === $this->driver ) {
			try {
				$info = $this->redis->info();
				$out  = '';
				foreach ( (array) $info as $k => $v ) {
					$out .= $k . ':' . $v . "\n";
				}
				return $out;
			} catch ( \Throwable $e ) {
				$this->fail( $e->getMessage() );
				return '';
			}
		}
		$res = $this->cmd( array( 'INFO' ) );
		return is_string( $res ) ? $res : '';
	}

	/**
	 * @return int number of keys in the current DB matching our prefix.
	 */
	public function count_keys( $prefix ) {
		if ( ! $this->is_connected() ) {
			return 0;
		}
		$count   = 0;
		$pattern = $prefix . '*';
		$cursor  = '0';
		if ( 'phpredis' === $this->driver ) {
			try {
				$it = null;
				while ( false !== ( $keys = $this->redis->scan( $it, $pattern, 1000 ) ) ) {
					$count += count( $keys );
					if ( 0 === (int) $it ) {
						break;
					}
				}
			} catch ( \Throwable $e ) {
				$this->fail( $e->getMessage() );
			}
			return $count;
		}
		do {
			$res = $this->cmd( array( 'SCAN', $cursor, 'MATCH', $pattern, 'COUNT', '1000' ) );
			if ( ! is_array( $res ) || count( $res ) < 2 ) {
				break;
			}
			$cursor = (string) $res[0];
			$count += is_array( $res[1] ) ? count( $res[1] ) : 0;
		} while ( '0' !== $cursor && ! $this->dead );
		return $count;
	}

	public function close() {
		if ( 'phpredis' === $this->driver && $this->redis ) {
			try {
				$this->redis->close();
			} catch ( \Throwable $e ) {
				// ignore.
			}
		}
		if ( is_resource( $this->stream ) ) {
			@fclose( $this->stream ); // phpcs:ignore
		}
		$this->stream    = null;
		$this->redis     = null;
		$this->connected = false;
	}

	// ---------------------------------------------------------------------
	// Pure-PHP RESP protocol.
	// ---------------------------------------------------------------------

	/**
	 * Send one command (array of arguments) and read one reply.
	 *
	 * @param string[] $args
	 * @return mixed string|int|array|true|null  (null = nil reply / error)
	 */
	private function cmd( array $args ) {
		if ( ! is_resource( $this->stream ) ) {
			return null;
		}
		$payload = '*' . count( $args ) . "\r\n";
		foreach ( $args as $a ) {
			$a        = (string) $a;
			$payload .= '$' . strlen( $a ) . "\r\n" . $a . "\r\n";
		}
		if ( false === $this->write( $payload ) ) {
			$this->fail( 'write failed' );
			return null;
		}
		return $this->read_reply();
	}

	private function write( $buf ) {
		$len     = strlen( $buf );
		$written = 0;
		while ( $written < $len ) {
			$n = @fwrite( $this->stream, substr( $buf, $written ) ); // phpcs:ignore
			if ( false === $n || 0 === $n ) {
				$meta = stream_get_meta_data( $this->stream );
				if ( ! empty( $meta['timed_out'] ) ) {
					return false;
				}
				return false;
			}
			$written += $n;
		}
		return true;
	}

	/**
	 * @return mixed
	 */
	private function read_reply() {
		$line = $this->read_line();
		if ( false === $line || '' === $line ) {
			$this->fail( 'empty reply' );
			return null;
		}
		$type    = $line[0];
		$payload = substr( $line, 1 );
		switch ( $type ) {
			case '+': // simple string.
				return ( 'PONG' === $payload ) ? 'PONG' : $payload;
			case '-': // error.
				$this->last_error = 'redis error: ' . $payload;
				return null;
			case ':': // integer.
				return (int) $payload;
			case '$': // bulk string.
				$len = (int) $payload;
				if ( -1 === $len ) {
					return null;
				}
				$data = $this->read_bytes( $len + 2 ); // include trailing CRLF.
				if ( false === $data ) {
					$this->fail( 'short bulk read' );
					return null;
				}
				return substr( $data, 0, $len );
			case '*': // array.
				$count = (int) $payload;
				if ( -1 === $count ) {
					return null;
				}
				$arr = array();
				for ( $i = 0; $i < $count; $i++ ) {
					$arr[] = $this->read_reply();
					if ( $this->dead ) {
						return null;
					}
				}
				return $arr;
			default:
				$this->fail( 'unknown reply type: ' . $type );
				return null;
		}
	}

	private function read_line() {
		$line = @fgets( $this->stream ); // phpcs:ignore
		if ( false === $line ) {
			return false;
		}
		return rtrim( $line, "\r\n" );
	}

	private function read_bytes( $n ) {
		$buf = '';
		while ( strlen( $buf ) < $n ) {
			$chunk = @fread( $this->stream, $n - strlen( $buf ) ); // phpcs:ignore
			if ( false === $chunk || '' === $chunk ) {
				$meta = stream_get_meta_data( $this->stream );
				if ( ! empty( $meta['timed_out'] ) ) {
					return false;
				}
				return false;
			}
			$buf .= $chunk;
		}
		return $buf;
	}
}
