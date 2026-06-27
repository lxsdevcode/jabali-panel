package commands

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"git.linux-hosting.co.il/shukivaknin/jabali2/internal/filesafe"
)

// TestMaildirOpenRefusesSymlinkEscape covers the contract openMaildirFileInStaging
// relies on (Gitea #477): a real message under the staging root opens, but a
// symlink (message or cur/new dir) escaping the root is refused — so a malicious
// migration source cannot turn mailbox import into a root-side host file read.
func TestMaildirOpenRefusesSymlinkEscape(t *testing.T) {
	staging := t.TempDir()
	outside := t.TempDir()
	scope := filesafe.NewScopeForTest("job", "migration", staging)

	// Real message inside cur/ opens and reads back.
	cur := filepath.Join(staging, "dom", "user", "cur")
	if err := os.MkdirAll(cur, 0o755); err != nil {
		t.Fatal(err)
	}
	msg := filepath.Join(cur, "1.eml")
	if err := os.WriteFile(msg, []byte("Message-ID: <a@b>\n\nhi"), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := scope.OpenInScope(msg, os.O_RDONLY, 0)
	if err != nil {
		t.Fatalf("real message should open: %v", err)
	}
	b, _ := io.ReadAll(f)
	_ = f.Close()
	if len(b) == 0 {
		t.Error("expected message bytes")
	}

	// A symlinked message pointing at a host file is refused.
	secret := filepath.Join(outside, "shadow")
	if err := os.WriteFile(secret, []byte("root:x:0:0"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(cur, "evil.eml")); err != nil {
		t.Fatal(err)
	}
	if _, err := scope.OpenInScope(filepath.Join(cur, "evil.eml"), os.O_RDONLY, 0); err == nil {
		t.Error("symlinked message must be refused")
	}

	// A symlinked cur/ directory escaping the root is refused.
	link := filepath.Join(staging, "dom", "user2", "cur")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if _, err := scope.OpenInScope(filepath.Join(link, "shadow"), os.O_RDONLY, 0); err == nil {
		t.Error("message under a symlinked cur/ must be refused")
	}
}
