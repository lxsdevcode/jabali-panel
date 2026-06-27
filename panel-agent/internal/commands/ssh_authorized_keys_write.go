package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os/user"
	"path/filepath"

	"git.linux-hosting.co.il/shukivaknin/jabali2/agentwire"
	"git.linux-hosting.co.il/shukivaknin/jabali2/internal/filesafe"
)

// sshAuthorizedKeysWriteParams is the input shape for ssh.authorized_keys.write.
type sshAuthorizedKeysWriteParams struct {
	Username string   `json:"username"`
	Keys     []string `json:"keys"`
}

// sshAuthorizedKeysWriteResponse is the output shape for ssh.authorized_keys.write.
type sshAuthorizedKeysWriteResponse struct {
	Username string `json:"username"`
	KeyCount int    `json:"key_count"`
	Written  bool   `json:"written"`
}

func sshAuthorizedKeysWriteHandler(ctx context.Context, params json.RawMessage) (any, error) {
	var p sshAuthorizedKeysWriteParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &agentwire.AgentError{
			Code:    agentwire.CodeInvalidArgument,
			Message: fmt.Sprintf("failed to parse params: %v", err),
		}
	}

	// Validate username and resolve user's homedir
	u, err := user.Lookup(p.Username)
	if err != nil {
		return nil, &agentwire.AgentError{
			Code:    agentwire.CodeInvalidArgument,
			Message: fmt.Sprintf("user %q not found: %v", p.Username, err),
		}
	}

	// Every .ssh/authorized_keys op runs through a filesafe scope rooted at
	// the user's home (Gitea #467). The agent is root, so if .ssh (or any
	// component) is a tenant-planted symlink, plain os.Stat/Chown/Chmod/
	// WriteFile/Rename would follow it and redirect root-side ownership and
	// managed-key writes outside the real home. openat2 RESOLVE_BENEATH
	// refuses any symlink in the path.
	scope, serr := filesafe.NewScope(p.Username, p.Username, []string{u.HomeDir})
	if serr != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInternal, Message: "scope init: " + serr.Error()}
	}

	sshDir := filepath.Join(u.HomeDir, ".ssh")
	authorizedKeysPath := filepath.Join(sshDir, "authorized_keys")

	// Preserve any operator / break-glass keys: jabali only owns its
	// delimited block (ADR — incident 2026-05-17). Read current file,
	// re-render only the managed block, keep everything else verbatim.
	existing, rerr := readAuthorizedKeys(scope, authorizedKeysPath)
	if rerr != nil {
		return nil, rerr
	}
	content := applyManagedBlock(existing, p.Keys)
	if werr := writeAuthorizedKeysAtomic(scope, u, sshDir, authorizedKeysPath, content); werr != nil {
		return nil, werr
	}

	return &sshAuthorizedKeysWriteResponse{
		Username: p.Username,
		KeyCount: len(p.Keys),
		Written:  true,
	}, nil
}

// ensureSSHDir ensures the .ssh directory exists with owner and mode 0700,
// escape-proof: created via openat2 MkdirInScope, then its mode and owner are
// fixed through a directory fd (fchmod/fchown) so a symlinked .ssh is refused
// rather than followed (Gitea #467).
func ensureSSHDir(scope *filesafe.Scope, sshDir string, u *user.User) *agentwire.AgentError {
	if _, err := scope.MkdirInScope(sshDir, false, 0o700); err != nil {
		return &agentwire.AgentError{
			Code:    agentwire.CodeInternal,
			Message: fmt.Sprintf("failed to create .ssh dir: %v", err),
		}
	}
	d, err := scope.OpenDirInScope(sshDir)
	if err != nil {
		return &agentwire.AgentError{
			Code:    agentwire.CodeInternal,
			Message: fmt.Sprintf("failed to open .ssh dir: %v", err),
		}
	}
	defer d.Close()
	uid, gid := parseUID(u)
	if err := d.Chown(uid, gid); err != nil {
		return &agentwire.AgentError{
			Code:    agentwire.CodeInternal,
			Message: fmt.Sprintf("failed to chown .ssh dir: %v", err),
		}
	}
	if err := d.Chmod(0o700); err != nil {
		return &agentwire.AgentError{
			Code:    agentwire.CodeInternal,
			Message: fmt.Sprintf("failed to chmod .ssh dir: %v", err),
		}
	}
	return nil
}

// parseUID converts user.User to numeric uid/gid.
func parseUID(u *user.User) (int, int) {
	var uid, gid int
	if _, err := fmt.Sscanf(u.Uid, "%d", &uid); err != nil {
		uid = -1
	}
	if _, err := fmt.Sscanf(u.Gid, "%d", &gid); err != nil {
		gid = -1
	}
	return uid, gid
}

func init() {
	Default.Register("ssh.authorized_keys.write", sshAuthorizedKeysWriteHandler)
}
