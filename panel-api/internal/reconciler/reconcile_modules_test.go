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

// moduleStubAgent returns a canned system.module.status per key and records
// install calls. status[key] = {"installed":bool,"active":bool}.
type moduleStubAgent struct {
	status       map[string]map[string]any
	installCalls int
}

func (m *moduleStubAgent) Call(_ context.Context, method string, params any) (json.RawMessage, error) {
	key := ""
	if mp, ok := params.(map[string]any); ok {
		key, _ = mp["key"].(string)
	}
	switch method {
	case "system.module.status":
		st := m.status[key]
		if st == nil {
			st = map[string]any{"installed": false, "active": false}
		}
		return json.Marshal(st)
	case "system.module.install":
		m.installCalls++
		return json.Marshal(m.status[key])
	default:
		return nil, nil
	}
}

var _ agent.AgentInterface = (*moduleStubAgent)(nil)

func discardReconciler(ag agent.AgentInterface) *Reconciler {
	return &Reconciler{agent: ag, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func up() map[string]any   { return map[string]any{"installed": true, "active": true} }
func down() map[string]any { return map[string]any{"installed": false, "active": false} }
func inactive() map[string]any {
	return map[string]any{"installed": true, "active": false}
}

// convergeModule (no dependency) must dispatch for the not-fully-up states and
// short-circuit when installed+active. Asserted on the synchronous backoff
// record (set iff an install was decided), avoiding a race with the detached
// install goroutine.
func TestConvergeModuleDecision(t *testing.T) {
	cases := []struct {
		name         string
		status       map[string]any
		wantDispatch bool
	}{
		{"installed+active converged", up(), false},
		{"missing", down(), true},
		{"installed but inactive (the bug)", inactive(), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := discardReconciler(&moduleStubAgent{status: map[string]map[string]any{"dns": tc.status}})
			r.convergeModule(context.Background(), "dns", "")

			r.moduleInstallMu.Lock()
			_, dispatched := r.moduleInstallAttempt["dns"]
			r.moduleInstallMu.Unlock()
			if dispatched != tc.wantDispatch {
				t.Errorf("dispatch = %v, want %v", dispatched, tc.wantDispatch)
			}
		})
	}
}

// A module with an unmet dependency must NOT dispatch and must NOT burn its
// backoff (so it installs promptly once the dependency converges). Once the
// dependency is up, the module dispatches normally.
func TestConvergeModuleDependency(t *testing.T) {
	// mail needs dns; dns is down → mail must not dispatch, no backoff recorded.
	r := discardReconciler(&moduleStubAgent{status: map[string]map[string]any{
		"dns":  down(),
		"mail": down(),
	}})
	r.convergeModule(context.Background(), "mail", "dns")
	r.moduleInstallMu.Lock()
	_, recorded := r.moduleInstallAttempt["mail"]
	r.moduleInstallMu.Unlock()
	if recorded {
		t.Error("mail with unmet dns dependency must not record a backoff attempt")
	}

	// dns up, mail down → mail dispatches.
	r2 := discardReconciler(&moduleStubAgent{status: map[string]map[string]any{
		"dns":  up(),
		"mail": down(),
	}})
	r2.convergeModule(context.Background(), "mail", "dns")
	r2.moduleInstallMu.Lock()
	_, dispatched := r2.moduleInstallAttempt["mail"]
	r2.moduleInstallMu.Unlock()
	if !dispatched {
		t.Error("mail with satisfied dns dependency (mail down) must dispatch")
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
	r.moduleInstallMu.Lock()
	r.moduleInstallAttempt["dns"] = time.Now().Add(-moduleInstallRetryInterval - time.Minute)
	r.moduleInstallMu.Unlock()
	if !r.moduleInstallDue("dns") {
		t.Fatal("attempt after the retry window should be due again")
	}
}
