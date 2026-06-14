package commands

import (
	"os"
	"path/filepath"
	"testing"
)

// looksLikeMailMaildir must accept a subfolder-only mailbox (empty
// INBOX, mail only in a Maildir++ .<Sub>) — the multi-mailbox restore
// regression where info@ (all mail in Junk) was silently dropped.
func TestLooksLikeMailMaildir_SubfolderOnly(t *testing.T) {
	mk := func(t *testing.T, parts ...string) string {
		t.Helper()
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(append([]string{root}, parts...)...), 0o700); err != nil {
			t.Fatal(err)
		}
		return root
	}

	t.Run("root cur", func(t *testing.T) {
		d := mk(t, "cur")
		if got, ok := looksLikeMailMaildir(d); !ok || got != d {
			t.Fatalf("root cur: got (%q,%v), want (%q,true)", got, ok, d)
		}
	})
	t.Run("subfolder only (empty INBOX)", func(t *testing.T) {
		d := mk(t, ".Junk", "new") // no root cur/new — the bug case
		if got, ok := looksLikeMailMaildir(d); !ok || got != d {
			t.Fatalf("subfolder-only: got (%q,%v), want (%q,true)", got, ok, d)
		}
	})
	t.Run("DA Maildir subdir", func(t *testing.T) {
		d := mk(t, "Maildir", "new")
		want := filepath.Join(d, "Maildir")
		if got, ok := looksLikeMailMaildir(d); !ok || got != want {
			t.Fatalf("DA: got (%q,%v), want (%q,true)", got, ok, want)
		}
	})
	t.Run("not a maildir", func(t *testing.T) {
		d := mk(t, "random")
		if got, ok := looksLikeMailMaildir(d); ok {
			t.Fatalf("non-maildir: got (%q,%v), want ('',false)", got, ok)
		}
	})
}
