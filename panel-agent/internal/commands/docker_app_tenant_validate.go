package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// M49 (GH #170) — tenant-install safety gate. Even though panel-api injects the
// hardening profile into the rendered compose, the agent independently
// validates the FULLY-RESOLVED config (`docker compose config`) before bringing
// a tenant app up. Defense in depth: a buggy/compromised panel cannot get a
// privileged container, an out-of-allowlist capability, or a host bind-mount
// past the agent. The catalog templates are admin/tenant-neutral, so this gate
// is the load-bearing check that a tenant install is actually contained.

// composeConfigService is the slice of `docker compose config --format json`
// we care about. Unknown fields are ignored.
type composeConfigService struct {
	Privileged bool     `json:"privileged"`
	CapAdd     []string `json:"cap_add"`
	Volumes    []struct {
		Type   string `json:"type"`
		Source string `json:"source"`
		Target string `json:"target"`
	} `json:"volumes"`
}

type composeConfigDoc struct {
	Services map[string]composeConfigService `json:"services"`
}

// normalizeCap upper-cases and strips a leading CAP_ so "cap_chown",
// "CHOWN" and "CAP_CHOWN" all compare equal.
func normalizeCap(c string) string {
	c = strings.ToUpper(strings.TrimSpace(c))
	return strings.TrimPrefix(c, "CAP_")
}

// validateTenantCompose rejects a resolved compose config that a tenant must
// never be able to run: any privileged service, any cap_add outside the
// catalog-verified allowlist, or any host bind-mount whose source escapes the
// app's own data tree (dataRoot). Pure (operates on the JSON bytes) so it is
// unit-tested without docker. Returns nil when the config is safe.
func validateTenantCompose(configJSON []byte, allowedCaps []string, dataRoot string) error {
	var doc composeConfigDoc
	if err := json.Unmarshal(configJSON, &doc); err != nil {
		return fmt.Errorf("parse compose config: %w", err)
	}
	if len(doc.Services) == 0 {
		return fmt.Errorf("compose config has no services")
	}
	allow := make(map[string]bool, len(allowedCaps))
	for _, c := range allowedCaps {
		allow[normalizeCap(c)] = true
	}
	root := filepath.Clean(dataRoot)
	for name, svc := range doc.Services {
		if svc.Privileged {
			return fmt.Errorf("service %q requests privileged: forbidden for tenant installs", name)
		}
		for _, c := range svc.CapAdd {
			if !allow[normalizeCap(c)] {
				return fmt.Errorf("service %q requests capability %q outside the tenant allowlist", name, c)
			}
		}
		for _, v := range svc.Volumes {
			if v.Type != "bind" {
				continue // named/anonymous volumes are docker-managed — fine
			}
			src := filepath.Clean(v.Source)
			if src != root && !strings.HasPrefix(src, root+string(filepath.Separator)) {
				return fmt.Errorf("service %q bind-mounts %q outside the app data tree %q: forbidden for tenant installs", name, v.Source, root)
			}
		}
	}
	return nil
}

// runTenantComposeValidation resolves the on-disk compose (dir) via
// `docker compose config --format json` and runs validateTenantCompose.
// Called by the install handler before `up` when the install is tenant-owned.
func runTenantComposeValidation(ctx context.Context, dir string, allowedCaps []string) error {
	cmd := exec.CommandContext(ctx, "docker", "compose", "config", "--format", "json")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker compose config failed: %v: %s", err, lastNonEmptyLines(string(out), 5))
	}
	return validateTenantCompose(out, allowedCaps, dir)
}
