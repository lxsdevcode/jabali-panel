package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"git.jabali-panel.com/shukivaknin/jabali2/agentwire"
)

// phpVersionReloadParams is the input shape for php.version.reload.
type phpVersionReloadParams struct {
	Version string `json:"version"`
}

// phpVersionReloadResponse is the output shape for php.version.reload.
type phpVersionReloadResponse struct {
	Version string `json:"version"`
	Message string `json:"message"`
}

func phpVersionReloadHandler(ctx context.Context, params json.RawMessage) (any, error) {
	if params == nil || len(params) == 0 {
		return nil, &agentwire.AgentError{
			Code:    agentwire.CodeInvalidArgument,
			Message: "version parameter required",
		}
	}

	var p phpVersionReloadParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &agentwire.AgentError{
			Code:    agentwire.CodeInvalidArgument,
			Message: fmt.Sprintf("failed to parse params: %v", err),
		}
	}

	// Validate version format
	if !versionRegex.MatchString(p.Version) {
		return nil, &agentwire.AgentError{
			Code:    agentwire.CodeInvalidArgument,
			Message: fmt.Sprintf("invalid version format: %q (expected X.Y)", p.Version),
		}
	}

	// Check if version is supported
	if !isVersionSupported(p.Version) {
		return nil, &agentwire.AgentError{
			Code:    agentwire.CodeInvalidArgument,
			Message: fmt.Sprintf("unsupported version: %q", p.Version),
		}
	}

	// GH#180: the global php<v>-fpm.service is MASKED on every host
	// (ADR-0025 — per-user slices). `systemctl reload-or-restart
	// php8.4-fpm.service` therefore always fails with "Unit ... is
	// masked", which the panel surfaced as a 500 (a Cloudflare 5xx page
	// to proxied operators). The real workers are the per-user
	// jabali-fpm@<user>.service instances whose user is pinned to this
	// PHP version. Reload those; the masked global is a no-op.
	reloaded, failures := reloadVersionFPMUnits(ctx, p.Version)
	if len(failures) > 0 {
		return nil, &agentwire.AgentError{
			Code:    agentwire.CodeInternal,
			Message: fmt.Sprintf("failed to reload PHP %s FPM workers: %s", p.Version, strings.Join(failures, "; ")),
		}
	}
	msg := fmt.Sprintf("reloaded %d PHP %s FPM worker(s)", len(reloaded), p.Version)
	if len(reloaded) == 0 {
		msg = fmt.Sprintf("no active PHP %s FPM workers to reload", p.Version)
	}
	return phpVersionReloadResponse{
		Version: p.Version,
		Message: msg,
	}, nil
}

// reloadVersionFPMUnits reload-or-restarts every running
// jabali-fpm@<user>.service whose user is pinned to `version`
// (/etc/jabali-panel/user-phpver/<user>). Returns the units reloaded and
// any genuine failures. The masked global php<v>-fpm.service is
// intentionally not touched (ADR-0025).
func reloadVersionFPMUnits(ctx context.Context, version string) (reloaded, failures []string) {
	out, err := runSystemctl(ctx, "list-units", "jabali-fpm@*.service", "--state=running", "--no-legend", "--plain", "--no-pager")
	if err != nil {
		return nil, []string{fmt.Sprintf("list jabali-fpm units: %v", err)}
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		unit := fields[0]
		at := strings.Index(unit, "@")
		dot := strings.LastIndex(unit, ".service")
		if at < 0 || dot <= at+1 {
			continue
		}
		user := unit[at+1 : dot]
		if !userPinnedToVersion(user, version) {
			continue
		}
		if cout, cerr := runSystemctl(ctx, "reload-or-restart", unit); cerr != nil {
			failures = append(failures, fmt.Sprintf("%s: %v: %s", unit, cerr, strings.TrimSpace(string(cout))))
			continue
		}
		reloaded = append(reloaded, unit)
	}
	return reloaded, failures
}

func init() {
	Default.Register("php.version.reload", phpVersionReloadHandler)
}
