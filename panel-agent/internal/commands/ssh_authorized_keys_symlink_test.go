package commands

import (
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"

	"git.jabali-panel.com/shukivaknin/jabali2/internal/filesafe"
)

// TestSSHAuthorizedKeysSymlinkSafe covers the .ssh symlink-escape hardening
// (Gitea #467): a normal write renders the managed block into the real home,
// but if .ssh is a tenant-planted symlink the root-side write is refused and
// nothing is written through the symlink target.
func TestSSHAuthorizedKeysSymlinkSafe(t *testing.T) {
	home := t.TempDir()
	outside := t.TempDir()
	scope := filesafe.NewScopeForTest("u", "alice", home)
	// Uid/Gid "-1" → parseUID returns -1,-1 → fchown is a no-op (test runs
	// unprivileged); the path-safety behavior is what's under test.
	u := &user.User{Uid: "-1", Gid: "-1"}

	sshDir := filepath.Join(home, ".ssh")
	ak := filepath.Join(sshDir, "authorized_keys")

	// Positive: fresh write creates .ssh + authorized_keys with the block.
	content := applyManagedBlock("", []string{"ssh-ed25519 AAAATESTKEY alice@host"})
	if err := writeAuthorizedKeysAtomic(scope, u, sshDir, ak, content); err != nil {
		t.Fatalf("write should succeed on a clean home: %v", err)
	}
	b, err := os.ReadFile(ak)
	if err != nil {
		t.Fatalf("read back authorized_keys: %v", err)
	}
	if !strings.Contains(string(b), "AAAATESTKEY") || !strings.Contains(string(b), akBeginMarker) {
		t.Fatalf("managed block not written: %q", b)
	}

	// Negative: replace .ssh with a symlink to an outside dir. The write must
	// be refused and the symlink target must stay untouched.
	if err := os.RemoveAll(sshDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, sshDir); err != nil {
		t.Fatal(err)
	}
	if werr := writeAuthorizedKeysAtomic(scope, u, sshDir, ak, "ssh-rsa EVIL\n"); werr == nil {
		t.Fatal("write through symlinked .ssh must be refused")
	}
	if _, e := os.Stat(filepath.Join(outside, "authorized_keys")); e == nil {
		t.Fatal("root write leaked through the .ssh symlink into the target dir")
	}
}
