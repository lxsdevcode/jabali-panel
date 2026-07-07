package hestiacp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPeekUserName(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "user.conf"),
		[]byte("SUSPENDED='no'\nFNAME='John'\nLNAME='Doe'\nCONTACT='john@x.com'\n"), 0o644)
	fn, ln := PeekUserName(dir)
	if fn != "John" || ln != "Doe" {
		t.Errorf("got (%q,%q), want (John,Doe)", fn, ln)
	}
	// absent file → empty, no panic
	if f, l := PeekUserName(t.TempDir()); f != "" || l != "" {
		t.Errorf("absent user.conf should be empty, got (%q,%q)", f, l)
	}
}
