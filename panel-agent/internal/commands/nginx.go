package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"git.linux-hosting.co.il/shukivaknin/jabali2/agentwire"
)

// nginxTestResponse is the output shape for nginx.test.
type nginxTestResponse struct {
	Valid bool `json:"valid"`
}

// nginxReloadResponse is the output shape for nginx.reload.
type nginxReloadResponse struct {
	Reloaded bool `json:"reloaded"`
}

// nginx.test is hot — the admin panel's global server-status poller
// hits it on every Server Settings / Updates / Security page tab the
// operator has open (5â30s cadence). On hosts with the CrowdSec
// AppSec acquis loaded across many vhosts, a single `nginx -t` runs
// ~19s at >60% of one core because Coraza compiles the full CRS rule
// set per-vhost during the parse phase. Caching for 60s drops that
// load by ~95% without giving up correctness: nginx config doesn't
// change between unrelated polls, and any verb that actually mutates
// vhost config (nginx.reload, domain.create, etc.) explicitly
// invalidates the cache below.
const nginxTestTTL = 60 * time.Second

var (
	nginxTestMu     sync.Mutex
	nginxTestCached *nginxTestResponse
	nginxTestAt     time.Time
)

// invalidateNginxTestCache forces the next nginx.test call to re-run
// `nginx -t`. Called from nginx.reload and from any other handler in
// this package that mutates files nginx parses (e.g. domain.create
// rendering vhosts). The cache only memoises the syntactic VALID/
// INVALID verdict; an out-of-band config change between calls is
// still picked up after the TTL elapses.
func invalidateNginxTestCache() {
	nginxTestMu.Lock()
	nginxTestCached = nil
	nginxTestAt = time.Time{}
	nginxTestMu.Unlock()
}

func nginxTestHandler(ctx context.Context, params json.RawMessage) (any, error) {
	nginxTestMu.Lock()
	if nginxTestCached != nil && time.Since(nginxTestAt) < nginxTestTTL {
		cached := *nginxTestCached
		nginxTestMu.Unlock()
		return cached, nil
	}
	nginxTestMu.Unlock()

	testCmd := exec.CommandContext(ctx, "nginx", "-t")
	var combinedOutput bytes.Buffer
	testCmd.Stdout = &combinedOutput
	testCmd.Stderr = &combinedOutput

	if err := testCmd.Run(); err != nil {
		// Failures intentionally NOT cached â operator likely just
		// hand-edited a vhost and is staring at the panel waiting
		// for the next poll to confirm the fix. Caching a fail for
		// 60s would punish them. The successful 60s window is the
		// hot path we're optimising for.
		return nil, &agentwire.AgentError{
			Code:    agentwire.CodeInternal,
			Message: fmt.Sprintf("nginx test failed: %s", combinedOutput.String()),
		}
	}

	resp := nginxTestResponse{Valid: true}
	nginxTestMu.Lock()
	nginxTestCached = &resp
	nginxTestAt = time.Now()
	nginxTestMu.Unlock()
	return resp, nil
}

func nginxReloadHandler(ctx context.Context, params json.RawMessage) (any, error) {
	// First test nginx configuration
	_, err := nginxTestHandler(ctx, params)
	if err != nil {
		return nil, err
	}

	// Run systemctl reload nginx
	reloadCmd := exec.CommandContext(ctx, "systemctl", "reload", "nginx")
	var combinedOutput bytes.Buffer
	reloadCmd.Stdout = &combinedOutput
	reloadCmd.Stderr = &combinedOutput

	if err := reloadCmd.Run(); err != nil {
		return nil, &agentwire.AgentError{
			Code:    agentwire.CodeInternal,
			Message: fmt.Sprintf("systemctl reload nginx failed: %s", combinedOutput.String()),
		}
	}

	// Reload succeeded â the next nginx.test poll should re-validate
	// against whatever config is now live, not return the pre-reload
	// cached verdict.
	invalidateNginxTestCache()

	return nginxReloadResponse{Reloaded: true}, nil
}

func init() {
	Default.Register("nginx.test", nginxTestHandler)
	Default.Register("nginx.reload", nginxReloadHandler)
}
