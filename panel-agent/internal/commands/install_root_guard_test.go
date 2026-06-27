package commands

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAssertSafeInstallRoot covers the symlinked-install-root guard (GH #457):
// a planted symlink or non-directory must be refused before any root-side
// chown/rm, while a real directory or a not-yet-existing path is allowed.
func TestAssertSafeInstallRoot(t *testing.T) {
	base := t.TempDir()

	realDir := filepath.Join(base, "site")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := assertSafeInstallRoot(realDir); err != nil {
		t.Errorf("real dir should be allowed, got %v", err)
	}

	if err := assertSafeInstallRoot(filepath.Join(base, "does-not-exist")); err != nil {
		t.Errorf("missing path should be allowed (created later), got %v", err)
	}

	target := filepath.Join(base, "target")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "evil")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := assertSafeInstallRoot(link); err == nil {
		t.Error("symlinked install root must be refused")
	}

	file := filepath.Join(base, "afile")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := assertSafeInstallRoot(file); err == nil {
		t.Error("non-directory install root must be refused")
	}
}
