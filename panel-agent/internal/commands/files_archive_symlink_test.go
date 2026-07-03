package commands

import (
	"context"
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"git.jabali-panel.com/shukivaknin/jabali2/internal/filesafe"
)

// TestAddToTarSymlinkBodyNotLeaked covers the files.archive Lstat-to-open race
// hardening (Gitea #471): regular files inside the tenant home are archived
// with their content, but a symlink that points outside the home is stored as
// a symlink header only — its target's bytes are never streamed into the
// archive, so a swapped/escaping symlink cannot disclose host files.
func TestAddToTarSymlinkBodyNotLeaked(t *testing.T) {
	home := t.TempDir()
	outside := t.TempDir()

	secret := filepath.Join(outside, "shadow")
	const secretBody = "ROOT-SECRET-DO-NOT-LEAK"
	if err := os.WriteFile(secret, []byte(secretBody), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "ok.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(home, "evil")); err != nil {
		t.Fatal(err)
	}

	scope := filesafe.NewScopeForTest("u", "alice", home)
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := addToTar(context.Background(), tw, scope, home, filepath.Dir(home)); err != nil {
		t.Fatalf("addToTar: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	if strings.Contains(buf.String(), secretBody) {
		t.Fatal("archive leaked out-of-home symlink target content")
	}

	tr := tar.NewReader(&buf)
	var sawOK, sawEvilSymlink bool
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		switch {
		case strings.HasSuffix(hdr.Name, "ok.txt"):
			sawOK = true
			b, _ := io.ReadAll(tr)
			if string(b) != "hello" {
				t.Errorf("ok.txt body = %q, want hello", b)
			}
		case strings.HasSuffix(hdr.Name, "evil"):
			if hdr.Typeflag != tar.TypeSymlink {
				t.Errorf("evil should be archived as a symlink, got typeflag %d", hdr.Typeflag)
			}
			sawEvilSymlink = true
		}
	}
	if !sawOK {
		t.Error("regular file ok.txt missing from archive")
	}
	if !sawEvilSymlink {
		t.Error("symlink entry missing from archive")
	}
}
