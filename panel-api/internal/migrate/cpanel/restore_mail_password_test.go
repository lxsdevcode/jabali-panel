package cpanel

import (
	"os"
	"path/filepath"
	"testing"
)

// normalizeBcryptHash strips a {SCHEME} tag and accepts only bcrypt variants
// Stalwart verifies — a non-bcrypt hash must be dropped (storing it would re-
// create the silent-lockout bug with a new value).
func TestNormalizeBcryptHash(t *testing.T) {
	cases := map[string]string{
		"{BLF-CRYPT}$2y$05$abcdefghijklmnopqrstuv": "$2y$05$abcdefghijklmnopqrstuv",
		"$2a$10$abcdefghijklmnopqrstuv":            "$2a$10$abcdefghijklmnopqrstuv",
		"$2b$12$abcdefghijklmnopqrstuv":            "$2b$12$abcdefghijklmnopqrstuv",
		"{SHA512-CRYPT}$6$rounds=5000$salt$hash":   "", // not bcrypt → drop
		"{PLAIN}hunter2":                           "",
		"plaintextnope":                            "",
		"":                                         "",
	}
	for in, want := range cases {
		if got := normalizeBcryptHash(in); got != want {
			t.Errorf("normalizeBcryptHash(%q) = %q, want %q", in, got, want)
		}
	}
}

// loadSourceMailPasswords parses a Hestia dovecot passwd-file into local→hash,
// keeping only Stalwart-verifiable bcrypt; missing file → empty.
func TestLoadSourceMailPasswords(t *testing.T) {
	dir := t.TempDir()
	confDir := filepath.Join(dir, "conf")
	if err := os.MkdirAll(confDir, 0o750); err != nil {
		t.Fatal(err)
	}
	content := "" +
		"info:{BLF-CRYPT}$2y$05$JH3fabcdefghijklmnopqr:1001:1001::/home\n" +
		"legacy:{SHA512-CRYPT}$6$rounds=5000$s$h:1002:1002::/home\n" +
		"\n" +
		"# comment line\n" +
		"admin:$2b$10$anotheranotheranotherx:1003:1003\n"
	if err := os.WriteFile(filepath.Join(confDir, "passwd"), []byte(content), 0o640); err != nil {
		t.Fatal(err)
	}

	got := loadSourceMailPasswords(dir)
	if got["info"] != "$2y$05$JH3fabcdefghijklmnopqr" {
		t.Errorf("info hash = %q", got["info"])
	}
	if got["admin"] != "$2b$10$anotheranotheranotherx" {
		t.Errorf("admin hash = %q", got["admin"])
	}
	if _, ok := got["legacy"]; ok {
		t.Error("legacy (SHA-CRYPT) must be dropped, not stored")
	}
	if len(got) != 2 {
		t.Errorf("want 2 usable hashes, got %d: %v", len(got), got)
	}

	// Missing file → empty map, no error.
	if m := loadSourceMailPasswords(t.TempDir()); len(m) != 0 {
		t.Errorf("missing passwd → want empty, got %v", m)
	}
}
