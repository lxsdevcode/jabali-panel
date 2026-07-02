package commands

import (
	"os"
	"path/filepath"
	"testing"

	"git.jabali-panel.com/shukivaknin/jabali2/internal/filesafe"
)

// TestMigrationRsyncDestContainment pins the destination-binding contract used
// by migration.rsync_remote_home (Gitea #465): the dest path must resolve
// inside the destination user's home, and a symlinked component pointing
// outside it must be refused before the root rsync + recursive chown.
func TestMigrationRsyncDestContainment(t *testing.T) {
	home := t.TempDir()                 // stands in for /home/<dest_user>
	outside := t.TempDir()              // another tenant / host path
	scope := filesafe.NewScopeForTest("u", "alice", home)

	// In-home path (not yet existing) resolves fine — restore creates it.
	if _, err := scope.Resolve(filepath.Join(home, "domains", "site")); err != nil {
		t.Errorf("in-home dest should resolve, got %v", err)
	}

	// A sibling/other-tenant path outside the scope is rejected.
	if _, err := scope.Resolve(filepath.Join(outside, "loot")); err == nil {
		t.Error("dest outside the user home must be rejected")
	}

	// A symlinked component that escapes the home must be refused.
	link := filepath.Join(home, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if _, err := scope.Resolve(filepath.Join(link, "loot")); err == nil {
		t.Error("symlink-escaping dest must be rejected")
	}
}
