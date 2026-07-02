package commands

import "strings"

import "testing"

// GH #621: cloning must not carry the source install's JABALI_CACHE_* block
// into the clone's wp-config (it pins the source's cache prefix + ACL, causing
// cross-install cache bleed). The clone flow calls stripWPCacheBlock; verify it
// removes the managed block while preserving the rest of wp-config.
func TestStripWPCacheBlock_ClonePath(t *testing.T) {
	src := "<?php\n" +
		"define( 'DB_NAME', 'wp_src' );\n" +
		"define( 'DB_USER', 'u' );\n" +
		wpConfigBeginMarker + "\n" +
		"if ( ! defined( 'JABALI_CACHE_PREFIX' ) )   define( 'JABALI_CACHE_PREFIX', 'shuki:src123' );\n" +
		"if ( ! defined( 'JABALI_CACHE_USER' ) )     define( 'JABALI_CACHE_USER', 'wp_shuki' );\n" +
		wpConfigEndMarker + "\n" +
		"/* That's all, stop editing! */\n"

	got := stripWPCacheBlock(src)

	if strings.Contains(got, "JABALI_CACHE_PREFIX") || strings.Contains(got, wpConfigBeginMarker) {
		t.Fatalf("cache block not stripped:\n%s", got)
	}
	for _, keep := range []string{"DB_NAME", "DB_USER", "stop editing"} {
		if !strings.Contains(got, keep) {
			t.Errorf("stripped too much — missing %q", keep)
		}
	}
	// Idempotent + safe on a config without the block.
	if out := stripWPCacheBlock("<?php\ndefine( 'DB_NAME', 'x' );\n"); !strings.Contains(out, "DB_NAME") {
		t.Errorf("no-block config mangled: %q", out)
	}
}
