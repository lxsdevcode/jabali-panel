package filesafe

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

// These tests prove the openat2 primitives close the parent-directory
// symlink-escape class (Gitea #421/#422/#423/#424/#427/#428): a tenant who owns
// their docroot plants a symlink there pointing OUT of scope, and every
// root-side op through that symlink must FAIL with the victim left untouched.

// escapeEnv builds a docroot (the scope base) plus an out-of-scope victim dir,
// and plants two escaping symlinks inside the docroot:
//
//	docroot/abs -> /abs/path/to/victim   (absolute  -> RESOLVE_BENEATH rejects)
//	docroot/up  -> ../victim             (upward     -> RESOLVE_BENEATH rejects)
//
// plus one LEGIT in-scope relative symlink that must keep working:
//
//	docroot/good -> sub                  (relative, stays beneath -> allowed)
func escapeEnv(t *testing.T) (scope *Scope, docroot, victim string) {
	t.Helper()
	// Skip if the kernel is pre-5.6 (no openat2).
	if _, err := unix.Openat2(unix.AT_FDCWD, ".", &unix.OpenHow{Flags: unix.O_RDONLY, Resolve: unix.RESOLVE_BENEATH}); err != nil && errors.Is(err, unix.ENOSYS) {
		t.Skip("openat2 not supported on this kernel")
	}
	root := t.TempDir()
	docroot = filepath.Join(root, "home")
	victim = filepath.Join(root, "victim")
	for _, d := range []string{docroot, victim, filepath.Join(docroot, "sub")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Victim secret the attacker must never read or clobber.
	if err := os.WriteFile(filepath.Join(victim, "secret"), []byte("top-secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, filepath.Join(docroot, "abs")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../victim", filepath.Join(docroot, "up")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("sub", filepath.Join(docroot, "good")); err != nil {
		t.Fatal(err)
	}
	return NewScopeForTest("u1", "u1", docroot), docroot, victim
}

func victimUntouched(t *testing.T, victim string) {
	t.Helper()
	if b, err := os.ReadFile(filepath.Join(victim, "secret")); err != nil || string(b) != "top-secret" {
		t.Fatalf("victim secret was modified: err=%v content=%q", err, string(b))
	}
	for _, planted := range []string{"pwn", "clone", "newdir"} {
		if _, err := os.Lstat(filepath.Join(victim, planted)); err == nil {
			t.Fatalf("attacker planted %q inside the victim dir — escape!", planted)
		}
	}
}

func TestOpenat2_WriteThroughParentSymlink_Rejected(t *testing.T) {
	scope, docroot, victim := escapeEnv(t)
	for _, via := range []string{"abs", "up"} {
		target := filepath.Join(docroot, via, "pwn")
		if f, err := scope.CreateExclInScope(target, 0o600); err == nil {
			f.Close()
			t.Errorf("CreateExclInScope through %q symlink succeeded — escape!", via)
		}
	}
	victimUntouched(t, victim)
}

func TestOpenat2_ReadThroughParentSymlink_Rejected(t *testing.T) {
	scope, docroot, victim := escapeEnv(t)
	for _, via := range []string{"abs", "up"} {
		target := filepath.Join(docroot, via, "secret")
		if f, err := scope.OpenInScope(target, os.O_RDONLY, 0); err == nil {
			f.Close()
			t.Errorf("OpenInScope through %q symlink read an out-of-scope file — escape!", via)
		}
	}
	victimUntouched(t, victim)
}

func TestOpenat2_StatAndListThroughParentSymlink_Rejected(t *testing.T) {
	scope, docroot, victim := escapeEnv(t)
	for _, via := range []string{"abs", "up"} {
		if _, err := scope.StatInScope(filepath.Join(docroot, via, "secret")); err == nil {
			t.Errorf("StatInScope through %q leaked out-of-scope metadata", via)
		}
		if _, err := scope.ReadDirInScope(filepath.Join(docroot, via)); err == nil {
			t.Errorf("ReadDirInScope through %q listed an out-of-scope dir", via)
		}
	}
	victimUntouched(t, victim)
}

func TestOpenat2_RenameThroughParentSymlink_Rejected(t *testing.T) {
	scope, docroot, victim := escapeEnv(t)
	// A real in-scope source to try to move out.
	src := filepath.Join(docroot, "src.txt")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, via := range []string{"abs", "up"} {
		if err := scope.RenameInScope(src, filepath.Join(docroot, via, "pwn")); err == nil {
			t.Errorf("RenameInScope into %q symlink succeeded — escape!", via)
		}
	}
	victimUntouched(t, victim)
}

func TestOpenat2_RemoveAllThroughParentSymlink_Rejected(t *testing.T) {
	scope, _, victim := escapeEnv(t)
	// Deleting through the symlink must not reach into the victim. Removing the
	// symlink leaf itself is fine; removing a path BEYOND it must fail.
	for _, via := range []string{"abs", "up"} {
		_ = scope.RemoveAllInScope(filepath.Join(victim, "..", "home", via, "secret"))
	}
	victimUntouched(t, victim)
}

func TestOpenat2_ChmodThroughParentSymlink_Rejected(t *testing.T) {
	scope, docroot, victim := escapeEnv(t)
	for _, via := range []string{"abs", "up"} {
		if err := scope.ChmodInScope(filepath.Join(docroot, via, "secret"), 0o777); err == nil {
			t.Errorf("ChmodInScope through %q symlink succeeded — escape!", via)
		}
	}
	// The victim secret must still be 0600.
	if fi, err := os.Lstat(filepath.Join(victim, "secret")); err == nil && fi.Mode().Perm() != 0o600 {
		t.Fatalf("victim secret mode changed to %o — escape!", fi.Mode().Perm())
	}
	victimUntouched(t, victim)
}

func TestOpenat2_MkdirThroughParentSymlink_Rejected(t *testing.T) {
	scope, docroot, victim := escapeEnv(t)
	for _, via := range []string{"abs", "up"} {
		if _, err := scope.MkdirInScope(filepath.Join(docroot, via, "newdir"), false, 0o755); err == nil {
			t.Errorf("MkdirInScope through %q symlink succeeded — escape!", via)
		}
	}
	victimUntouched(t, victim)
}

func TestOpenat2_CopyTreeThroughParentSymlink_Rejected(t *testing.T) {
	scope, docroot, victim := escapeEnv(t)
	src := filepath.Join(docroot, "sub") // real in-scope dir
	for _, via := range []string{"abs", "up"} {
		if _, err := scope.CopyTreeInScope(src, filepath.Join(docroot, via, "clone"), os.Getuid(), os.Getgid()); err == nil {
			t.Errorf("CopyTreeInScope into %q symlink succeeded — escape!", via)
		}
	}
	victimUntouched(t, victim)
}

// Legit in-scope relative symlinks must KEEP working under RESOLVE_BENEATH —
// this is why we don't add RESOLVE_NO_SYMLINKS.
func TestOpenat2_InScopeRelativeSymlink_Allowed(t *testing.T) {
	scope, docroot, _ := escapeEnv(t)
	// docroot/good -> sub ; write a file beneath the symlinked dir.
	if f, err := scope.CreateExclInScope(filepath.Join(docroot, "good", "ok.txt"), 0o644); err != nil {
		t.Fatalf("write through legit in-scope relative symlink must succeed: %v", err)
	} else {
		f.Close()
	}
	// It must have landed in the real target dir.
	if _, err := os.Lstat(filepath.Join(docroot, "sub", "ok.txt")); err != nil {
		t.Fatalf("file did not land in the in-scope symlink target: %v", err)
	}
}
