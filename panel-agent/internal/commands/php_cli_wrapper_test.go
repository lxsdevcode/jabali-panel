package commands

import (
	"os"
	"path/filepath"
	"testing"
)

// TestReplaceCLISymlink — idempotent + atomic-replace behavior of the
// per-user php wrapper symlink (GH #184).
func TestReplaceCLISymlink(t *testing.T) {
	dir := t.TempDir()
	tA := filepath.Join(dir, "php8.3")
	tB := filepath.Join(dir, "php8.4")
	for _, p := range []string{tA, tB} {
		if err := os.WriteFile(p, []byte("#!/bin/true\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(dir, "php")

	// Create.
	if err := replaceCLISymlink(link, tA); err != nil {
		t.Fatalf("create: %v", err)
	}
	if got, _ := os.Readlink(link); got != tA {
		t.Fatalf("after create, link -> %q want %q", got, tA)
	}

	// Idempotent: same target, no error, no temp left behind.
	if err := replaceCLISymlink(link, tA); err != nil {
		t.Fatalf("idempotent: %v", err)
	}
	if _, err := os.Lstat(link + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("temp symlink leaked")
	}

	// Re-point to a new version (atomic replace over the live link).
	if err := replaceCLISymlink(link, tB); err != nil {
		t.Fatalf("repoint: %v", err)
	}
	if got, _ := os.Readlink(link); got != tB {
		t.Fatalf("after repoint, link -> %q want %q", got, tB)
	}
}

// TestEnsureUserCLIPHP_Validation — bad input is rejected before any
// filesystem mutation; the symlink target must be a real php<v> binary.
func TestEnsureUserCLIPHP_Validation(t *testing.T) {
	cases := []struct{ user, version string }{
		{"bad user", "8.4"},      // space → invalid username
		{"alice", "8"},           // not X.Y
		{"alice", "8.4; rm -rf"}, // injection-shaped
		{"alice", "99.9"},        // valid shape but /usr/bin/php99.9 absent
	}
	for _, c := range cases {
		if err := ensureUserCLIPHP(c.user, c.version); err == nil {
			t.Errorf("ensureUserCLIPHP(%q,%q): expected error, got nil", c.user, c.version)
		}
	}
}

// TestRemoveUserCLIPHP_BadUser — invalid username rejected.
func TestRemoveUserCLIPHP_BadUser(t *testing.T) {
	if err := removeUserCLIPHP("bad user"); err == nil {
		t.Error("expected error for invalid username")
	}
}
