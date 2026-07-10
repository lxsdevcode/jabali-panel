package commands

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"git.jabali-panel.com/shukivaknin/jabali2/agentwire"
)

// Unknown/absent module keys must be rejected with CodeInvalidArgument BEFORE
// any shell-out — never let an arbitrary string reach install.sh.
func TestSystemModuleKeyAllowlist(t *testing.T) {
	ctx := context.Background()
	for _, bad := range []string{"", "core", "nginx", "dns; rm -rf /", "DNS", "postgres"} {
		raw, _ := json.Marshal(moduleStatusRequest{Key: bad})

		if _, err := systemModuleStatusHandler(ctx, raw); !isInvalidArg(err) {
			t.Errorf("status key %q: want CodeInvalidArgument, got %v", bad, err)
		}
		if _, err := systemModuleInstallHandler(ctx, raw); !isInvalidArg(err) {
			t.Errorf("install key %q: want CodeInvalidArgument, got %v", bad, err)
		}
	}
}

// Every allowlisted key has a probe entry, and status returns a well-formed
// response for it (Installed/Active reflect the local host — we only assert the
// key echoes back and the call doesn't error).
func TestSystemModuleStatusValidKeys(t *testing.T) {
	ctx := context.Background()
	for key := range moduleInstallKeys {
		if _, ok := moduleProbes[key]; !ok {
			t.Errorf("allowlisted key %q has no moduleProbes entry", key)
		}
		raw, _ := json.Marshal(moduleStatusRequest{Key: key})
		out, err := systemModuleStatusHandler(ctx, raw)
		if err != nil {
			t.Errorf("status %q errored: %v", key, err)
			continue
		}
		resp, ok := out.(moduleStatusResponse)
		if !ok {
			t.Fatalf("status %q: unexpected response type %T", key, out)
		}
		if resp.Key != key {
			t.Errorf("status %q: response key = %q", key, resp.Key)
		}
	}
}

// A tooling module (no service) reports Active == Installed rather than probing
// systemd for a unit that doesn't exist.
func TestProbeModuleQuotaActiveMirrorsInstalled(t *testing.T) {
	resp := probeModule(context.Background(), "quota")
	if resp.Active != resp.Installed {
		t.Errorf("quota: Active (%v) should mirror Installed (%v) — no service", resp.Active, resp.Installed)
	}
}

func isInvalidArg(err error) bool {
	var ae *agentwire.AgentError
	return errors.As(err, &ae) && ae.Code == agentwire.CodeInvalidArgument
}

// Disable must reject unknown keys before touching systemd.
func TestSystemModuleDisableKeyAllowlist(t *testing.T) {
	ctx := context.Background()
	for _, bad := range []string{"", "core", "nginx", "dns; rm -rf /"} {
		raw, _ := json.Marshal(moduleStatusRequest{Key: bad})
		if _, err := systemModuleDisableHandler(ctx, raw); !isInvalidArg(err) {
			t.Errorf("disable key %q: want CodeInvalidArgument, got %v", bad, err)
		}
	}
}

// The disable unit lists must never contain baseline plumbing or security
// guardrails — stopping these from a UI toggle breaks the host (DNS resolution)
// or is fail-open (firewall) / a posture regression (MAC, integrity).
func TestModuleDisableUnitsExcludeGuardrails(t *testing.T) {
	forbidden := map[string]bool{
		"pdns-recursor": true, // owns 127.0.0.1:53 — host DNS resolution
		"ufw":           true, // firewall — dropping it is fail-open
		"apparmor":      true, // live MAC confinement
		"aide":          true, // integrity monitoring
	}
	for key, units := range moduleDisableUnits {
		for _, u := range units {
			if forbidden[u] {
				t.Errorf("module %q disable list must NOT stop guardrail unit %q", key, u)
			}
		}
	}
	// dns disables only the authoritative server, never the recursor.
	if len(moduleDisableUnits["dns"]) != 1 || moduleDisableUnits["dns"][0] != "pdns" {
		t.Errorf("dns disable units = %v, want [pdns] only", moduleDisableUnits["dns"])
	}
	// Every disable key must be a known module key.
	for key := range moduleDisableUnits {
		if !moduleInstallKeys[key] {
			t.Errorf("moduleDisableUnits has non-module key %q", key)
		}
	}
}
