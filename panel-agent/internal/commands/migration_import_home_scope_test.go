package commands

import (
	"os"
	"path/filepath"
	"testing"

	"git.jabali-panel.com/shukivaknin/jabali2/internal/filesafe"
)

// TestMigrationImportHomeContainment pins the src + dest binding contracts used
// by migration.import_home (Gitea #468): the source must resolve inside the
// migration staging root and the destination inside the dest user's home, and
// a symlinked component escaping either root must be refused before the root
// rsync + recursive chown.
func TestMigrationImportHomeContainment(t *testing.T) {
	staging := t.TempDir() // stands in for /var/lib/jabali-migrations
	home := t.TempDir()    // stands in for /home/<dest_user>
	outside := t.TempDir()

	srcScope := filesafe.NewScopeForTest("job", "migration", staging)
	destScope := filesafe.NewScopeForTest("u", "alice", home)

	// In-scope paths resolve.
	if _, err := srcScope.Resolve(filepath.Join(staging, "job1", "extracted")); err != nil {
		t.Errorf("in-staging src should resolve: %v", err)
	}
	if _, err := destScope.Resolve(filepath.Join(home, "public_html")); err != nil {
		t.Errorf("in-home dest should resolve: %v", err)
	}

	// A symlink inside staging that points outside must be refused (would
	// otherwise let rsync pull arbitrary host files into the tenant home).
	srcLink := filepath.Join(staging, "escape")
	if err := os.Symlink(outside, srcLink); err != nil {
		t.Fatal(err)
	}
	if _, err := srcScope.Resolve(filepath.Join(srcLink, "loot")); err == nil {
		t.Error("symlink-escaping src must be rejected")
	}

	// A symlinked dest component escaping the home must be refused.
	destLink := filepath.Join(home, "escape")
	if err := os.Symlink(outside, destLink); err != nil {
		t.Fatal(err)
	}
	if _, err := destScope.Resolve(filepath.Join(destLink, "sub")); err == nil {
		t.Error("symlink-escaping dest must be rejected")
	}
}
