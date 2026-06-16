package fsperm

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// TestRepairDocrootGroup_SetgidAndSymlinkSafe verifies the happy path
// (group + setgid applied to the chain + subtree) and the security
// property (a symlink inside the tree is never followed, so its target
// outside the docroot is left untouched). Runs as the test user: it
// chowns to the caller's own gid, which is permitted without root.
func TestRepairDocrootGroup_SetgidAndSymlinkSafe(t *testing.T) {
	gid := os.Getgid()

	home := t.TempDir()
	docroot := filepath.Join(home, "domains", "d.example", "public_html")
	sub := filepath.Join(docroot, "wp-content", "uploads")
	if err := os.MkdirAll(sub, 0o750); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(sub, "img.jpg")
	if err := os.WriteFile(file, []byte("x"), 0o640); err != nil {
		t.Fatal(err)
	}

	// A target OUTSIDE the docroot, plus a symlink to it inside the tree.
	outside := t.TempDir()
	outsideDir := filepath.Join(outside, "secret")
	if err := os.Mkdir(outsideDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideDir, filepath.Join(sub, "evil")); err != nil {
		t.Fatal(err)
	}

	if err := RepairDocrootGroup(home, docroot, gid); err != nil {
		t.Fatalf("RepairDocrootGroup: %v", err)
	}

	// Chain dirs + subtree dirs must be setgid and group=gid.
	for _, d := range []string{
		filepath.Join(home, "domains"),
		filepath.Join(home, "domains", "d.example"),
		docroot,
		filepath.Join(docroot, "wp-content"),
		sub,
	} {
		fi, err := os.Stat(d)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode()&os.ModeSetgid == 0 {
			t.Errorf("%s: setgid bit not set (mode %v)", d, fi.Mode())
		}
		if st, ok := fi.Sys().(*syscall.Stat_t); ok && int(st.Gid) != gid {
			t.Errorf("%s: gid=%d want %d", d, st.Gid, gid)
		}
	}

	// Regular file: group flipped, no setgid (we only setgid dirs).
	if st, err := os.Stat(file); err == nil {
		if sys, ok := st.Sys().(*syscall.Stat_t); ok && int(sys.Gid) != gid {
			t.Errorf("file gid=%d want %d", sys.Gid, gid)
		}
	}

	// SECURITY: the symlink's target outside the tree must be untouched —
	// never setgid, mode still 0700.
	ofi, err := os.Stat(outsideDir)
	if err != nil {
		t.Fatal(err)
	}
	if ofi.Mode()&os.ModeSetgid != 0 {
		t.Fatalf("symlink target outside docroot was setgid'd — symlink was followed! mode=%v", ofi.Mode())
	}
	if ofi.Mode().Perm() != 0o700 {
		t.Fatalf("symlink target perm changed to %v — symlink was followed", ofi.Mode().Perm())
	}
}

// TestRepairDocrootGroup_RejectsOutsideHome ensures a docroot not under
// the anchor home is refused rather than walked.
func TestRepairDocrootGroup_RejectsOutsideHome(t *testing.T) {
	home := t.TempDir()
	other := t.TempDir()
	if err := RepairDocrootGroup(home, other, os.Getgid()); err == nil {
		t.Fatal("expected error for docroot outside home, got nil")
	}
}
