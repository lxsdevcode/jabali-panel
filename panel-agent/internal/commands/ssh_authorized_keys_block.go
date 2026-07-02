package commands

import (
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"git.jabali-panel.com/shukivaknin/jabali2/agentwire"
	"git.jabali-panel.com/shukivaknin/jabali2/internal/filesafe"
)

// jabali manages ONLY the delimited block below in a user's
// authorized_keys — never the whole file. Any operator / break-glass
// key outside the markers is preserved across every reconcile pass.
//
// Incident 2026-05-17: the old write/delete handlers full-replaced /
// os.Remove'd the entire authorized_keys, so the reconciler's
// every-tick "user has no jabali keys → delete" wiped operator SSH
// access on every jabali host (mx/.150). Marker-scoped edits fix it.
const (
	akBeginMarker = "# >>> jabali managed (do not edit this block) >>>"
	akEndMarker   = "# <<< jabali managed <<<"
)

// stripManagedBlock returns existing with the jabali marker block (and
// its contents) removed, leaving every other line untouched.
func stripManagedBlock(existing string) string {
	if existing == "" {
		return ""
	}
	out := make([]string, 0, 8)
	inBlock := false
	for _, ln := range strings.Split(existing, "\n") {
		t := strings.TrimSpace(ln)
		if t == akBeginMarker {
			inBlock = true
			continue
		}
		if t == akEndMarker {
			inBlock = false
			continue
		}
		if inBlock {
			continue
		}
		out = append(out, ln)
	}
	return strings.Join(out, "\n")
}

// applyManagedBlock returns the new authorized_keys content: all
// non-jabali lines from existing preserved verbatim, followed by a
// freshly-rendered jabali block (omitted entirely when keys is empty).
// Idempotent.
func applyManagedBlock(existing string, keys []string) string {
	base := strings.TrimRight(stripManagedBlock(existing), " \t\n")

	var b strings.Builder
	if base != "" {
		b.WriteString(base)
		b.WriteString("\n")
	}
	wrote := false
	for _, k := range keys {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		if !wrote {
			b.WriteString(akBeginMarker)
			b.WriteString("\n")
			wrote = true
		}
		b.WriteString(k)
		b.WriteString("\n")
	}
	if wrote {
		b.WriteString(akEndMarker)
		b.WriteString("\n")
	}
	return b.String()
}

// writeAuthorizedKeysAtomic writes content to the user's authorized_keys via a
// temp file + atomic rename, owned by the user, mode 0600. Every filesystem op
// goes through the escape-proof filesafe scope so a symlinked .ssh component
// cannot redirect the root-side write (Gitea #467). Shared by write + delete.
func writeAuthorizedKeysAtomic(scope *filesafe.Scope, u *user.User, sshDir, path, content string) *agentwire.AgentError {
	if aerr := ensureSSHDir(scope, sshDir, u); aerr != nil {
		return aerr
	}
	tmp := filepath.Join(sshDir, "authorized_keys.tmp")
	// Clear any stale temp from a previous crash so CreateExcl can't EEXIST.
	if err := scope.RemoveInScope(tmp, false); err != nil && !os.IsNotExist(err) {
		return &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("clear stale temp: %v", err)}
	}
	f, err := scope.CreateExclInScope(tmp, 0o600)
	if err != nil {
		return &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("create temp: %v", err)}
	}
	uid, gid := parseUID(u)
	cleanup := func() { _ = f.Close(); _ = scope.RemoveInScope(tmp, false) }
	if _, err := f.WriteString(content); err != nil {
		cleanup()
		return &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("write temp: %v", err)}
	}
	if err := f.Chown(uid, gid); err != nil {
		cleanup()
		return &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("chown temp: %v", err)}
	}
	if err := f.Close(); err != nil {
		_ = scope.RemoveInScope(tmp, false)
		return &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("close temp: %v", err)}
	}
	if err := scope.RenameInScope(tmp, path); err != nil {
		_ = scope.RemoveInScope(tmp, false)
		return &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("rename authorized_keys: %v", err)}
	}
	return nil
}

// readAuthorizedKeys returns the current file content, "" if absent. The open
// is escape-proof (openat2 RESOLVE_BENEATH) so a symlinked .ssh/authorized_keys
// cannot redirect a root-side read outside the home (Gitea #467).
func readAuthorizedKeys(scope *filesafe.Scope, path string) (string, *agentwire.AgentError) {
	f, err := scope.OpenInScope(path, os.O_RDONLY, 0)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("read authorized_keys: %v", err)}
	}
	defer f.Close()
	b, rerr := io.ReadAll(f)
	if rerr != nil {
		return "", &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("read authorized_keys: %v", rerr)}
	}
	return string(b), nil
}
