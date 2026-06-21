package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"git.linux-hosting.co.il/shukivaknin/jabali2/agentwire"
)

// php.cli_default_set — set or clear a user's explicit CLI default PHP version
// (GH #256). PHP version is per-domain, but a user has a single `php` on PATH;
// this lets the user pick which version a bare `php`/composer/wp-cli resolves
// to, independent of any domain. Writes the choice to
// /etc/jabali-panel/user-phpcli/<user> (the host-side source of truth the pin
// resolver honours over the pool-derived auto version), then re-points the
// wrapper. An empty version clears the choice → revert to auto.

type phpCLIDefaultParams struct {
	Username string `json:"username"`
	Version  string `json:"version"` // "" = clear (revert to auto)
}

func phpCLIDefaultSetHandler(_ context.Context, params json.RawMessage) (any, error) {
	var p phpCLIDefaultParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: fmt.Sprintf("parse: %v", err)}
	}
	if !phpPoolUsernameRegex.MatchString(p.Username) {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: fmt.Sprintf("invalid username %q", p.Username)}
	}
	if p.Version != "" && !phpVersionRegex.MatchString(p.Version) {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: fmt.Sprintf("invalid version %q", p.Version)}
	}
	if p.Version != "" {
		// Reject a version that isn't actually installed, so the bare `php`
		// can't be pointed at a missing binary.
		if fi, err := os.Stat("/usr/bin/php" + p.Version); err != nil || fi.IsDir() {
			return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: fmt.Sprintf("php %s not installed", p.Version)}
		}
	}

	choicePath := filepath.Join(userCLIChoiceRoot(), p.Username)
	if p.Version == "" {
		if err := os.Remove(choicePath); err != nil && !os.IsNotExist(err) {
			return nil, &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("clear choice: %v", err)}
		}
	} else {
		if err := os.MkdirAll(userCLIChoiceRoot(), 0o755); err != nil {
			return nil, &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("mkdir choice root: %v", err)}
		}
		if err := os.WriteFile(choicePath, []byte(p.Version+"\n"), 0o644); err != nil {
			return nil, &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("write choice: %v", err)}
		}
	}

	// Re-point the wrapper (effective = choice ?? auto pin). Skipped silently
	// for a system/no-home account.
	if err := ensureUserCLIPHP(p.Username, ""); err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("re-pin cli php: %v", err)}
	}
	return map[string]any{"username": p.Username, "version": p.Version}, nil
}

func init() {
	Default.Register("php.cli_default_set", phpCLIDefaultSetHandler)
}
