package main

import (
	"os"
	"path/filepath"
	"testing"
)

// JAB-7: the recovery CSV holds plaintext temp passwords and must be created
// safely — no world-writable parent, no following/truncating an existing file
// or symlink.
func TestOpenSecretOutputFile(t *testing.T) {
	dir := t.TempDir()

	// happy path: fresh file, mode 0600
	p := filepath.Join(dir, "tokens.csv")
	f, err := openSecretOutputFile(p)
	if err != nil {
		t.Fatalf("fresh create failed: %v", err)
	}
	f.Close()
	fi, _ := os.Stat(p)
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("mode = %o, want 600", fi.Mode().Perm())
	}

	// existing file refused (no truncate)
	if _, err := openSecretOutputFile(p); err == nil {
		t.Error("expected refusal for existing file")
	}

	// symlink refused (not followed)
	target := filepath.Join(dir, "victim")
	link := filepath.Join(dir, "link.csv")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if _, err := openSecretOutputFile(link); err == nil {
		t.Error("expected refusal for pre-seeded symlink")
	}
	if _, err := os.Lstat(target); err == nil {
		t.Error("symlink target was created — symlink was followed")
	}

	// world-writable parent refused
	ww := filepath.Join(dir, "wwdir")
	if err := os.Mkdir(ww, 0o777); err != nil {
		t.Fatal(err)
	}
	_ = os.Chmod(ww, 0o777)
	if _, err := openSecretOutputFile(filepath.Join(ww, "x.csv")); err == nil {
		t.Error("expected refusal under world-writable parent")
	}
}
