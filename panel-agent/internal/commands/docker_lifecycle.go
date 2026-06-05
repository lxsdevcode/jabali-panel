// docker_lifecycle.go — opt-in install / disable verbs for the M48
// docker app marketplace. Mirrors panel-agent/internal/commands/
// db_postgres_lifecycle.go pattern:
//
//   docker.install — sources install.sh and runs install_docker_engine.
//                    Idempotent; safe to re-run after a failed flip.
//   docker.disable — systemctl disable --now docker; data under
//                    /var/lib/jabali/docker-apps/ left intact for a
//                    future re-enable.
//   docker.status  — installed (binary present) + active (daemon up)
//                    + version string.
//
// Called by panel-api when the operator flips
// server_settings.docker_marketplace_enabled in Server Settings.
package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"git.linux-hosting.co.il/shukivaknin/jabali2/agentwire"
)

type dockerStatusResponse struct {
	Installed bool   `json:"installed"`
	Active    bool   `json:"active"`
	Version   string `json:"version,omitempty"`
}

func dockerStatusHandler(ctx context.Context, _ json.RawMessage) (any, error) {
	resp := dockerStatusResponse{}
	if _, err := exec.LookPath("docker"); err == nil {
		resp.Installed = true
	}
	if out, err := exec.CommandContext(ctx, "systemctl", "is-active", "docker").Output(); err == nil {
		resp.Active = strings.TrimSpace(string(out)) == "active"
	}
	if resp.Installed {
		if out, err := exec.CommandContext(ctx, "docker", "--version").Output(); err == nil {
			resp.Version = strings.TrimSpace(string(out))
		}
	}
	return resp, nil
}

// dockerInstallHandler sources install.sh and runs the existing
// install_docker_engine function. install.sh's docker block is
// idempotent (checks for `docker` + the compose-plugin package
// before pulling apt) so re-runs after a partial failure are safe.
// We wrap in systemd-run --pipe to escape the jabali-agent mount
// namespace (PrivateTmp + ProtectKernel* break apt + dpkg + docker
// postinst the same way they break the postgres install — same
// rationale as db.postgres.install).
func dockerInstallHandler(ctx context.Context, _ json.RawMessage) (any, error) {
	if _, err := os.Stat(installShPath); err != nil {
		return nil, &agentwire.AgentError{
			Code:    agentwire.CodeInternal,
			Message: fmt.Sprintf("install.sh missing at %s", installShPath),
		}
	}
	cmd := exec.CommandContext(ctx, "systemd-run",
		"--pipe", "--wait", "--quiet", "--collect",
		"--unit=jabali-docker-install",
		"--service-type=oneshot",
		"--", "bash", "-c",
		"source "+installShPath+" && install_docker_engine && "+
			"systemctl enable docker >/dev/null 2>&1 && "+
			"systemctl start docker")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, &agentwire.AgentError{
			Code: agentwire.CodeInternal,
			Message: fmt.Sprintf("install_docker_engine failed: %v: %s",
				err, strings.TrimSpace(string(out))),
		}
	}
	return dockerStatusHandler(ctx, nil)
}

// dockerDisableHandler stops the docker daemon and masks the unit
// so it does not auto-restart on next boot. Data under
// /var/lib/jabali/docker-apps/ is intentionally left intact -- the
// operator may re-enable later. Per-app rows + volumes survive.
func dockerDisableHandler(ctx context.Context, _ json.RawMessage) (any, error) {
	for _, args := range [][]string{
		{"systemctl", "disable", "--now", "docker"},
		{"systemctl", "disable", "--now", "docker.socket"},
	} {
		// Fail-soft: a unit that was never installed returns exit 1
		// from systemctl; treat that as success.
		_ = exec.CommandContext(ctx, args[0], args[1:]...).Run()
	}
	return dockerStatusHandler(ctx, nil)
}

func init() {
	Default.Register("docker.install", dockerInstallHandler)
	Default.Register("docker.disable", dockerDisableHandler)
	Default.Register("docker.status", dockerStatusHandler)
}
