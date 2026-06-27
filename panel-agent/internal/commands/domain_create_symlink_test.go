package commands

import (
	"os"
	"path/filepath"
	"testing"

	"git.linux-hosting.co.il/shukivaknin/jabali2/internal/filesafe"
)

// TestDomainDocrootSymlinkSafe covers GH #476: docroot provisioning goes
// through a home-rooted scope, so a tenant-planted symlink at domains/<domain>/
// public_html is refused instead of redirecting the root mkdir/chown/chmod, and
// a docroot outside the tenant's own home is rejected.
func TestDomainDocrootSymlinkSafe(t *testing.T) {
	home := t.TempDir()
	outside := t.TempDir()
	scope := filesafe.NewScopeForTest("u", "alice", home)
	docRoot := filepath.Join(home, "domains", "example.com", "public_html")

	// Clean home: the docroot chain is created.
	if _, err := scope.MkdirInScope(docRoot, true, 0o755); err != nil {
		t.Fatalf("mkdir docroot on clean home: %v", err)
	}
	if fi, err := os.Stat(docRoot); err != nil || !fi.IsDir() {
		t.Fatalf("docroot not created: %v", err)
	}

	// A docroot outside the tenant home is rejected.
	if _, err := scope.MkdirInScope(filepath.Join(outside, "evil"), true, 0o755); err == nil {
		t.Error("docroot outside the tenant home must be rejected")
	}

	// Hostile: replace 'domains' with a symlink to an outside dir.
	if err := os.RemoveAll(filepath.Join(home, "domains")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(home, "domains")); err != nil {
		t.Fatal(err)
	}
	if _, err := scope.MkdirInScope(docRoot, true, 0o755); err == nil {
		t.Error("mkdir through a symlinked 'domains' must be refused")
	}
	if _, err := os.Stat(filepath.Join(outside, "example.com")); err == nil {
		t.Error("root mkdir leaked through the domains symlink")
	}
}
