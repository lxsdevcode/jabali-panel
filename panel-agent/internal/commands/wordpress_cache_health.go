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
	"os/exec"
	"strconv"
	"strings"
	"time"

	"git.jabali-panel.com/shukivaknin/jabali2/agentwire"
)

type cacheHealthParams struct {
	OSUser      string `json:"os_user"`
	InstallPath string `json:"install_path"`
	Host        string `json:"host,omitempty"` // GH #620: for the TTFB probe
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

// wordpressCacheStatsHandler (GH #617) returns Redis cache stats as JSON by
// running `wp jabali-cache stats` as the tenant (which holds the ACL creds).
func wordpressCacheStatsHandler(ctx context.Context, params json.RawMessage) (any, error) {
	var p cacheHealthParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: fmt.Sprintf("parse params: %v", err)}
	}
	if p.OSUser == "" || p.InstallPath == "" {
		return nil, csInvalidArg("os_user and install_path are required")
	}
	out, err := runWPAsTenantOut(ctx, p.OSUser, p.InstallPath, "jabali-cache", "stats")
	if err != nil {
		return map[string]any{"connected": false, "detail": strings.TrimSpace(out)}, nil
	}
	var stats map[string]any
	if jErr := json.Unmarshal([]byte(strings.TrimSpace(out)), &stats); jErr != nil {
		return map[string]any{"connected": false, "detail": "unparseable stats output"}, nil
	}
	return stats, nil
}

// wordpressCacheProbeHandler (GH #620) returns the site's active plugins + WP
// version so the panel advisor can recommend a cache profile + settings.
func wordpressCacheProbeHandler(ctx context.Context, params json.RawMessage) (any, error) {
	var p cacheHealthParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: fmt.Sprintf("parse params: %v", err)}
	}
	if p.OSUser == "" || p.InstallPath == "" {
		return nil, csInvalidArg("os_user and install_path are required")
	}
	pl, _ := runWPAsTenantOut(ctx, p.OSUser, p.InstallPath, "plugin", "list", "--status=active", "--field=name", "--format=json")
	ver, _ := runWPAsTenantOut(ctx, p.OSUser, p.InstallPath, "core", "version")
	var plugins []string
	_ = json.Unmarshal([]byte(strings.TrimSpace(pl)), &plugins)
	ttfbMs := 0
	if p.Host != "" && probeDomainRe.MatchString(strings.ToLower(p.Host)) {
		// Timed request through LOCAL nginx (--resolve pins to loopback, no
		// redirects — same SSRF posture as the warmup, GH #639).
		cctx, cancel := context.WithTimeout(ctx, 12*time.Second)
		out, _ := exec.CommandContext(cctx, "curl", "-kso", "/dev/null",
			"-w", "%{time_starttransfer}", "--max-time", "10",
			"--resolve", p.Host+":443:127.0.0.1", "https://"+p.Host+"/").Output()
		cancel()
		if secs, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64); err == nil {
			ttfbMs = int(secs * 1000)
		}
	}
	return map[string]any{"active_plugins": plugins, "wp_version": strings.TrimSpace(ver), "ttfb_ms": ttfbMs}, nil
}

func init() {
	Default.Register("wordpress.cache_health", wordpressCacheHealthHandler)
	Default.Register("wordpress.cache_probe", wordpressCacheProbeHandler)
	Default.Register("wordpress.cache_stats", wordpressCacheStatsHandler)
}
