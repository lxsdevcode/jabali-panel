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

// TestReplaceCLISymlink_RefusesRegularFile — a planted regular file at the
// wrapper path must NOT be overwritten (symlink-TOCTOU hardening).
func TestReplaceCLISymlink_RefusesRegularFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "php8.4")
	if err := os.WriteFile(target, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "php")
	if err := os.WriteFile(link, []byte("planted"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := replaceCLISymlink(link, target); err == nil {
		t.Error("expected refusal to overwrite a regular file")
	}
	// The planted file is untouched.
	if b, _ := os.ReadFile(link); string(b) != "planted" {
		t.Error("planted file was modified")
	}
}

// TestEnsureRootDir_RefusesSymlink — a symlink where a dir is expected is
// refused (never followed).
func TestEnsureRootDir_RefusesSymlink(t *testing.T) {
	dir := t.TempDir()
	elsewhere := filepath.Join(dir, "elsewhere")
	if err := os.Mkdir(elsewhere, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "asdir")
	if err := os.Symlink(elsewhere, link); err != nil {
		t.Fatal(err)
	}
	if err := ensureRootDir(link); err == nil {
		t.Error("expected refusal for a symlink standing in for a directory")
	}
}

// GH #256: every installed php<X.Y> CLI binary must be enumerated (so a user
// with multiple domains on different versions gets php8.3/php8.4/... wrappers),
// while suffixed variants (php8.4-fpm) are excluded.
func TestInstalledPHPCLIVersions(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"php8.3", "php8.4", "php8.4-fpm", "php", "phpize"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	got := installedPHPCLIVersions(dir)
	if len(got) != 2 || got["8.3"] == "" || got["8.4"] == "" {
		t.Fatalf("want {8.3,8.4}, got %v", got)
	}
	if _, ok := got["8.4-fpm"]; ok {
		t.Error("php8.4-fpm must be excluded")
	}
}

// GH #256: the explicit CLI choice file overrides, the pin file is the
// fallback, and garbage is ignored.
func TestUserCLIChoiceResolution(t *testing.T) {
	choiceRoot := t.TempDir()
	pinRoot := t.TempDir()
	t.Setenv("JABALI_PHP_CLI_CHOICE_ROOT", choiceRoot)
	t.Setenv("JABALI_PHP_VER_PIN_ROOT", pinRoot)

	if readUserCLIChoice("alice") != "" {
		t.Error("no choice file → empty")
	}
	os.WriteFile(filepath.Join(pinRoot, "alice"), []byte("8.4\n"), 0o644)
	if readUserPhpverPin("alice") != "8.4" {
		t.Error("pin file should read 8.4")
	}
	os.WriteFile(filepath.Join(choiceRoot, "alice"), []byte("8.3\n"), 0o644)
	if readUserCLIChoice("alice") != "8.3" {
		t.Error("choice file should read 8.3 (overrides pin)")
	}
	os.WriteFile(filepath.Join(choiceRoot, "bob"), []byte("garbage; rm -rf\n"), 0o644)
	if readUserCLIChoice("bob") != "" {
		t.Error("garbage choice must be ignored")
	}
}
