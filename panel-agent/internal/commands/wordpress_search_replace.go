package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"git.jabali-panel.com/shukivaknin/jabali2/agentwire"
)

// wordpress.search_replace (GH #647) — serialized-safe URL replacement on an
// imported WordPress site during a domain-change migration. Runs
// `wp search-replace <old> <new> --path=<docroot> --all-tables` AS THE OS USER
// (via buildSystemdRunCmd, same as wordpress.clone step 4) so it never runs as
// root and PHP-serialized option/meta data is rewritten correctly (the whole
// reason a naive SQL REPLACE corrupts a WordPress DB).
type wordpressSearchReplaceParams struct {
	OSUser  string `json:"os_user"`
	Path    string `json:"path"`     // docroot (absolute, must be under the user's home)
	OldURL  string `json:"old_url"`  // e.g. https://old.example.com
	NewURL  string `json:"new_url"`  // e.g. https://new.example.com
	DryRun  bool   `json:"dry_run,omitempty"`
}

func wordpressSearchReplaceHandler(ctx context.Context, params json.RawMessage) (any, error) {
	var p wordpressSearchReplaceParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: "malformed JSON: " + err.Error()}
	}
	if !looksLikeUnixUsername(p.OSUser) {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: "os_user looks unsafe"}
	}
	// Path must be an absolute docroot under the user's home — never root/system.
	if !filepath.IsAbs(p.Path) || strings.Contains(p.Path, "..") ||
		!strings.HasPrefix(filepath.Clean(p.Path), "/home/"+p.OSUser+"/") {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: "path must be an absolute docroot under the user's home"}
	}
	if !isHTTPURL(p.OldURL) || !isHTTPURL(p.NewURL) {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: "old_url and new_url must be http(s) URLs"}
	}

	args := []string{"wp", "search-replace", p.OldURL, p.NewURL, "--path=" + p.Path, "--all-tables", "--report-changed-only"}
	if p.DryRun {
		args = append(args, "--dry-run")
	}
	cmd := buildSystemdRunCmd(ctx, p.OSUser, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, &agentwire.AgentError{
			Code:    agentwire.CodeInternal,
			Message: fmt.Sprintf("wp search-replace failed: %v: %s", err, strings.TrimSpace(string(out))),
		}
	}
	return map[string]any{"ok": true, "output": strings.TrimSpace(string(out))}, nil
}

// isHTTPURL is a cheap allowlist: http(s)://host, no shell/whitespace chars.
func isHTTPURL(u string) bool {
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		return false
	}
	if strings.ContainsAny(u, " \t\n\r;&|`$()<>\"'\\") {
		return false
	}
	return len(u) <= 253+8
}

func init() {
	Default.Register("wordpress.search_replace", wordpressSearchReplaceHandler)
}
