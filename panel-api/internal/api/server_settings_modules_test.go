package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// moduleAgentStub returns per-key system.module.status and records installs.
type moduleAgentStub struct {
	mu       sync.Mutex
	status   map[string]map[string]any
	installs []string
}

func (m *moduleAgentStub) Call(_ context.Context, cmd string, params any) (json.RawMessage, error) {
	key := ""
	if mp, ok := params.(map[string]any); ok {
		key, _ = mp["key"].(string)
	}
	switch cmd {
	case "system.module.status":
		st := m.status[key]
		if st == nil {
			st = map[string]any{"installed": false, "active": false}
		}
		return json.Marshal(st)
	case "system.module.install":
		m.mu.Lock()
		m.installs = append(m.installs, key)
		m.mu.Unlock()
		return json.RawMessage(`{"installed":true,"active":true}`), nil
	default:
		return json.RawMessage(`{"ok":true}`), nil
	}
}

func (m *moduleAgentStub) installedKeys() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.installs...)
}

// GET /admin/settings/modules/status aggregates the agent probe for every
// installable module (and never api_enabled, which has no install path).
func TestModuleStatusEndpoint(t *testing.T) {
	ag := &moduleAgentStub{status: map[string]map[string]any{
		"dns":      {"installed": true, "active": true},
		"mail":     {"installed": true, "active": false},
		"security": {"installed": false, "active": false},
		"quota":    {"installed": true, "active": true},
	}}
	r := settingsRouter(true, &mockServerSettingsRepo{}, ag)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings/modules/status", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		Modules map[string]struct {
			Installed bool `json:"installed"`
			Active    bool `json:"active"`
		} `json:"modules"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

	assert.True(t, body.Modules["dns"].Installed && body.Modules["dns"].Active)
	assert.True(t, body.Modules["mail"].Installed && !body.Modules["mail"].Active)
	assert.False(t, body.Modules["security"].Installed)
	if _, ok := body.Modules["api"]; ok {
		t.Error("api must not appear in module status (no install path)")
	}
}

// POST /admin/settings/modules/install dispatches an install for a valid key
// and rejects unknown keys.
func TestModuleInstallEndpoint(t *testing.T) {
	ag := &moduleAgentStub{status: map[string]map[string]any{}}
	r := settingsRouter(true, &mockServerSettingsRepo{}, ag)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/admin/settings/modules/install",
		strings.NewReader(`{"key":"dns"}`)))
	require.Equal(t, http.StatusAccepted, rec.Code)

	// dispatch is a goroutine; the endpoint returns before it lands, so just
	// assert the request was accepted. Unknown key must be rejected outright.
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, httptest.NewRequest(http.MethodPost, "/api/v1/admin/settings/modules/install",
		strings.NewReader(`{"key":"api"}`)))
	assert.Equal(t, http.StatusBadRequest, rec2.Code)

	rec3 := httptest.NewRecorder()
	r.ServeHTTP(rec3, httptest.NewRequest(http.MethodPost, "/api/v1/admin/settings/modules/install",
		strings.NewReader(`{"key":"bogus"}`)))
	assert.Equal(t, http.StatusBadRequest, rec3.Code)
}
