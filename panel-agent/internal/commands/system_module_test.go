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
