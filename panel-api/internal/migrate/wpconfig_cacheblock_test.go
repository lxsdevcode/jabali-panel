package migrate

import (
	"strings"
	"testing"
)

func TestStripJabaliCacheBlock(t *testing.T) {
	in := `<?php
define( 'DB_NAME', 'wp' );
// BEGIN Jabali WP Cache
if ( ! defined( 'JABALI_CACHE_SOCKET' ) ) define( 'JABALI_CACHE_SOCKET', '/run/redis/redis.sock' );
if ( ! defined( 'JABALI_CACHE_PREFIX' ) ) define( 'JABALI_CACHE_PREFIX', 'src:01ABC' );
// END Jabali WP Cache
require_once ABSPATH . 'wp-settings.php';
`
	out := StripJabaliCacheBlock(in)
	if strings.Contains(out, "JABALI_CACHE_") || strings.Contains(out, "BEGIN Jabali WP Cache") {
		t.Fatalf("cache block not stripped:\n%s", out)
	}
	if !strings.Contains(out, "DB_NAME") || !strings.Contains(out, "wp-settings.php") {
		t.Fatalf("stripped too much:\n%s", out)
	}
	if StripJabaliCacheBlock(out) != out {
		t.Fatal("not idempotent")
	}
}
