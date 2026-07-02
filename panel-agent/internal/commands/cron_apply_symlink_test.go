package commands

import (
	"os"
	"path/filepath"
	"testing"

	"git.jabali-panel.com/shukivaknin/jabali2/internal/filesafe"
)

// TestCronUnitWriteSymlinkSafe covers GH #475: cron unit dirs/files are created
// through a home-rooted scope, so a tenant-planted symlink at .config (or any
// component) is refused instead of redirecting the root-side write.
func TestCronUnitWriteSymlinkSafe(t *testing.T) {
	home := t.TempDir()
	outside := t.TempDir()
	scope := filesafe.NewScopeForTest("u", "alice", home)
	unitsDir := filepath.Join(home, ".config", "systemd", "user")

	// Clean home: MkdirInScope builds the chain and writeUnitInScope writes.
	if _, err := scope.MkdirInScope(unitsDir, true, 0o755); err != nil {
		t.Fatalf("mkdir units dir on clean home: %v", err)
	}
	svc := filepath.Join(unitsDir, "jabali-cron-x.service")
	if err := writeUnitInScope(scope, svc, []byte("[Service]\n"), -1, -1); err != nil {
		t.Fatalf("write unit: %v", err)
	}
	b, err := os.ReadFile(svc)
	if err != nil || string(b) != "[Service]\n" {
		t.Fatalf("unit not written: %v %q", err, b)
	}

	// Hostile: replace .config with a symlink to an outside dir.
	if err := os.RemoveAll(filepath.Join(home, ".config")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(home, ".config")); err != nil {
		t.Fatal(err)
	}
	if _, err := scope.MkdirInScope(unitsDir, true, 0o755); err == nil {
		t.Error("mkdir through a symlinked .config must be refused")
	}
	if _, err := os.Stat(filepath.Join(outside, "systemd", "user")); err == nil {
		t.Error("root write leaked through the .config symlink")
	}
}
