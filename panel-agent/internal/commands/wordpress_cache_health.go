package commands

// wordpress_cache_health.go — GH #605. Lightweight drift check for a
// cache-enabled WordPress install. `wp jabali-cache verify` exercises the whole
// stack end-to-end AS THE TENANT: it fails (non-zero exit) if the plugin is
// inactive (unknown command), the object-cache drop-in is missing/broken, the
// JABALI_CACHE_* constants are gone, the Redis ACL user/keyspace is wrong, or
// the socket is unreachable. So one call detects every drift mode the panel's
// cache_enabled=true flag silently lies about. Detection only — repair is the
// panel's idempotent SetApplicationCache path, driven by `jabali app cache-doctor`.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"git.jabali-panel.com/shukivaknin/jabali2/agentwire"
)

type cacheHealthParams struct {
	OSUser      string `json:"os_user"`
	InstallPath string `json:"install_path"`
}

func wordpressCacheHealthHandler(ctx context.Context, params json.RawMessage) (any, error) {
	var p cacheHealthParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: fmt.Sprintf("parse params: %v", err)}
	}
	if p.OSUser == "" || p.InstallPath == "" {
		return nil, csInvalidArg("os_user and install_path are required")
	}

	out, err := runWPAsTenantOut(ctx, p.OSUser, p.InstallPath, "jabali-cache", "verify")
	healthy := err == nil
	detail := strings.TrimSpace(out)
	if !healthy && detail == "" {
		detail = err.Error()
	}
	return map[string]any{"healthy": healthy, "detail": detail}, nil
}

func init() {
	Default.Register("wordpress.cache_health", wordpressCacheHealthHandler)
}
