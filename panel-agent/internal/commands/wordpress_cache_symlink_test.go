package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// GH #411: the root agent must refuse a tenant-planted symlink at wp-config.php
// and never read/write through it.
func TestSetWPConfigCacheConstants_RefusesSymlink(t *testing.T) {
	dir := t.TempDir()
	victim := filepath.Join(dir, "victim-secret")
	if err := os.WriteFile(victim, []byte("VICTIM DB CREDS\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	install := filepath.Join(dir, "install")
	if err := os.Mkdir(install, 0o755); err != nil {
		t.Fatal(err)
	}
	// tenant points wp-config.php at the victim file
	if err := os.Symlink(victim, filepath.Join(install, "wp-config.php")); err != nil {
		t.Fatal(err)
	}
	err := setWPConfigCacheConstants(install, "/run/redis/redis.sock", 1, "u:1", "tok", "wp_u", true)
	if err == nil {
		t.Fatal("expected refusal for symlinked wp-config.php (LPE/read-any-file)")
	}
	// victim must be byte-identical — not read into our file, not overwritten.
	if b, _ := os.ReadFile(victim); string(b) != "VICTIM DB CREDS\n" {
		t.Fatalf("victim modified through symlink: %q", b)
	}
}

func TestSetWPConfigCacheConstants_RealFileInsertsBlock(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "wp-config.php")
	body := "<?php\n/* That's all, stop editing! */\nrequire_once ABSPATH . 'wp-settings.php';\n"
	if err := os.WriteFile(cfg, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := setWPConfigCacheConstants(dir, "/run/redis/redis.sock", 1, "shukivaknin:01ABC", "tok123", "wp_shukivaknin", true); err != nil {
		t.Fatalf("real wp-config.php should succeed: %v", err)
	}
	b, _ := os.ReadFile(cfg)
	if !strings.Contains(string(b), "JABALI_CACHE_PASSWORD") || !strings.Contains(string(b), "wp-settings.php") {
		t.Fatalf("managed block not inserted before wp-settings: %q", b)
	}
}
