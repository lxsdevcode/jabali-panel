package commands

import (
	"os"
	"path/filepath"
	"testing"
)

// Gitea #430 interim: secret files must be stamped 0600, never the default
// group-readable mode.
func TestSensitiveFileMode(t *testing.T) {
	cases := map[string]os.FileMode{
		"/home/u/site/wp-config.php": 0o600,
		"/home/u/app/.env":           0o600,
		"/home/u/site/index.php":     0o640,
		"/home/u/site/style.css":     0o640,
	}
	for p, want := range cases {
		if got := sensitiveFileMode(p, 0o640); got != want {
			t.Errorf("sensitiveFileMode(%q) = %o, want %o", p, got, want)
		}
	}
}

// hardenSensitiveFilesInScope tightens docroot wp-config.php / .env to 0600,
// leaves ordinary files alone, and refuses to follow a symlink (so a tenant
// can't redirect the root-side chmod onto a victim file).
func TestHardenSensitiveFilesInScope(t *testing.T) {
	dir := t.TempDir()
	wp := filepath.Join(dir, "wp-config.php")
	env := filepath.Join(dir, ".env")
	normal := filepath.Join(dir, "index.php")
	for _, f := range []string{wp, env, normal} {
		if err := os.WriteFile(f, []byte("x"), 0o640); err != nil {
			t.Fatal(err)
		}
	}

	hardenSensitiveFilesInScope("u", dir)

	if fi, _ := os.Stat(wp); fi.Mode().Perm() != 0o600 {
		t.Errorf("wp-config.php = %o, want 600", fi.Mode().Perm())
	}
	if fi, _ := os.Stat(env); fi.Mode().Perm() != 0o600 {
		t.Errorf(".env = %o, want 600", fi.Mode().Perm())
	}
	if fi, _ := os.Stat(normal); fi.Mode().Perm() != 0o640 {
		t.Errorf("index.php = %o, want 640 (unchanged)", fi.Mode().Perm())
	}
}

func TestHardenSensitiveFiles_RefusesSymlink(t *testing.T) {
	dir := t.TempDir()
	victim := filepath.Join(dir, "victim")
	if err := os.WriteFile(victim, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Tenant plants wp-config.php as a symlink to a victim file.
	if err := os.Symlink(victim, filepath.Join(dir, "wp-config.php")); err != nil {
		t.Fatal(err)
	}

	hardenSensitiveFilesInScope("u", dir)

	if fi, _ := os.Lstat(victim); fi.Mode().Perm() != 0o644 {
		t.Errorf("victim perms changed to %o — symlink was followed!", fi.Mode().Perm())
	}
}
