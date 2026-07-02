package commands

import (
	"os"
	"path/filepath"
	"testing"

	"git.jabali-panel.com/shukivaknin/jabali2/internal/filesafe"
)

// TestSSOWriteInstallPathGate covers the install-path binding writeSSOFile
// relies on (Gitea #470): the SSO file is executable PHP written by root into
// a tenant webroot, so the install path must resolve inside the tenant's home,
// and a symlinked component pointing outside it must be refused before any
// create/rename. This exercises the same scope.Resolve + CreateExclInScope gate
// writeSSOFile uses, against a temp home.
func TestSSOWriteInstallPathGate(t *testing.T) {
	home := t.TempDir()
	outside := t.TempDir()
	scope := filesafe.NewScopeForTest("u", "alice", home)

	// In-home webroot resolves and accepts an exclusive create.
	webroot := filepath.Join(home, "domains", "site", "public_html")
	if err := os.MkdirAll(webroot, 0o755); err != nil {
		t.Fatal(err)
	}
	resolved, err := scope.Resolve(webroot)
	if err != nil {
		t.Fatalf("in-home webroot should resolve: %v", err)
	}
	f, err := scope.CreateExclInScope(resolved+"/jabali-sso-test.php", 0o640)
	if err != nil {
		t.Fatalf("create in-home SSO file: %v", err)
	}
	_ = f.Close()

	// A symlinked webroot pointing outside the home must be refused.
	evil := filepath.Join(home, "evil")
	if err := os.Symlink(outside, evil); err != nil {
		t.Fatal(err)
	}
	if _, err := scope.Resolve(filepath.Join(evil, "public_html")); err == nil {
		t.Fatal("symlink-escaping install_path must be rejected")
	}
}
