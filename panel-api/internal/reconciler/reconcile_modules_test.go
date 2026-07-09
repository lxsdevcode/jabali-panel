package reconciler

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/agent"
)

// moduleStubAgent returns a canned system.module.status and records install calls.
type moduleStubAgent struct {
	status       map[string]any
	installCalls int
}

func (m *moduleStubAgent) Call(_ context.Context, method string, _ any) (json.RawMessage, error) {
	switch method {
	case "system.module.status":
		return json.Marshal(m.status)
	case "system.module.install":
		m.installCalls++
		return json.Marshal(m.status)
	default:
		return nil, nil
	}
}

var _ agent.AgentInterface = (*moduleStubAgent)(nil)

func discardReconciler(ag agent.AgentInterface) *Reconciler {
	return &Reconciler{agent: ag, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

// convergeModule must dispatch an install exactly for the not-fully-up states
// (missing, or installed-but-inactive) and short-circuit when installed+active.
// We assert on the synchronous backoff record (set iff an install was decided),
// which avoids racing the detached install goroutine.
func TestConvergeModuleDecision(t *testing.T) {
	cases := []struct {
		name         string
		status       map[string]any
		wantErr      bool // status probe fails → no decision recorded
		wantDispatch bool
	}{
		{"installed+active converged", map[string]any{"installed": true, "active": true}, false, false},
		{"missing", map[string]any{"installed": false, "active": false}, false, true},
		{"installed but inactive (the bug)", map[string]any{"installed": true, "active": false}, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := discardReconciler(&moduleStubAgent{status: tc.status})
			r.convergeModule(context.Background(), "dns")

			r.moduleInstallMu.Lock()
			_, dispatched := r.moduleInstallAttempt["dns"]
			r.moduleInstallMu.Unlock()

			if dispatched != tc.wantDispatch {
				t.Errorf("dispatch = %v, want %v", dispatched, tc.wantDispatch)
			}
		})
	}
}

// The backoff gate must reject a second install within the retry window and
// allow it again afterward.
func TestModuleInstallBackoff(t *testing.T) {
	r := &Reconciler{}
	if !r.moduleInstallDue("dns") {
		t.Fatal("first attempt should be due")
	}
	if r.moduleInstallDue("dns") {
		t.Fatal("second immediate attempt should be gated by backoff")
	}
	if !r.moduleInstallDue("quota") {
		t.Fatal("a different module must not be gated by dns's attempt")
	}
	// Simulate the window elapsing.
	r.moduleInstallMu.Lock()
	r.moduleInstallAttempt["dns"] = time.Now().Add(-moduleInstallRetryInterval - time.Minute)
	r.moduleInstallMu.Unlock()
	if !r.moduleInstallDue("dns") {
		t.Fatal("attempt after the retry window should be due again")
	}
}
