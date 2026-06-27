package commands

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLchownTreeDoesNotFollowSymlinks covers GH #479: a symlink planted in a
// Docker volume must not have its target re-owned — lchownTree re-owns the link
// itself and never dereferences it, and the walk never descends a symlinked
// directory out of the tree.
func TestLchownTreeDoesNotFollowSymlinks(t *testing.T) {
	vol := t.TempDir()
	outside := t.TempDir()

	// A sensitive host file outside the volume.
	victim := filepath.Join(outside, "shadow")
	if err := os.WriteFile(victim, []byte("root:x:0:0"), 0o600); err != nil {
		t.Fatal(err)
	}
	// A symlink inside the volume pointing at it (what a container could plant).
	if err := os.Symlink(victim, filepath.Join(vol, "evil")); err != nil {
		t.Fatal(err)
	}
	// A symlinked directory pointing outside the volume.
	if err := os.Symlink(outside, filepath.Join(vol, "evildir")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vol, "real.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Re-own to the current uid/gid (a no-op ownership-wise when unprivileged,
	// but it exercises the Lchown path on the symlink without error).
	if err := lchownTree(vol, os.Getuid(), os.Getgid()); err != nil {
		t.Fatalf("lchownTree: %v", err)
	}

	// The symlink must still be a symlink (not replaced/followed) and its
	// target file must still exist with its original mode (not descended into).
	fi, err := os.Lstat(filepath.Join(vol, "evil"))
	if err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("evil should remain a symlink: %v mode=%v", err, fi.Mode())
	}
	vi, err := os.Stat(victim)
	if err != nil {
		t.Fatal(err)
	}
	if vi.Mode().Perm() != 0o600 {
		t.Errorf("victim mode changed to %v — walk dereferenced the symlink", vi.Mode().Perm())
	}
}
