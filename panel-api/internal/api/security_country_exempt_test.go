package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/agent"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/auth"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/countryexempt"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/ginctx"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
)

// countryExemptRouter wires the AppSec+country-exemption routes against the
// mock agent + mock settings repo, with the background CIDR kick stubbed so
// tests never touch the network.
func countryExemptRouter(t *testing.T, mock agent.AgentInterface, repo *mockServerSettingsRepo) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	orig := countryExemptKick
	countryExemptKick = func(_ *slog.Logger, _ *countryexempt.Syncer, _ []string, _ bool) {}
	t.Cleanup(func() { countryExemptKick = orig })

	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.Use(func(c *gin.Context) {
		ginctx.SetClaims(c, &auth.AccessClaims{UserID: "test-admin", IsAdmin: true})
		c.Next()
	})
	RegisterSecurityAppSecRoutes(v1, mock, repo)
	return r
}

func TestCountryExempt_Get_ReturnsStoredCountries(t *testing.T) {

	repo := &mockServerSettingsRepo{getResult: &models.ServerSettings{CountryExemptCountries: "IL,US"}}
	r := countryExemptRouter(t, agent.NewMockClient(), repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/security/crowdsec/country-exemption", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, []any{"IL", "US"}, body["countries"])
}

func TestCountryExempt_Get_EmptyIsArrayNotNull(t *testing.T) {

	repo := &mockServerSettingsRepo{getResult: &models.ServerSettings{}}
	r := countryExemptRouter(t, agent.NewMockClient(), repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/security/crowdsec/country-exemption", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"countries":[]`)
}

func TestCountryExempt_Put_InvalidCountry_NoAgentCall(t *testing.T) {

	mock := agent.NewMockClient()
	repo := &mockServerSettingsRepo{getResult: &models.ServerSettings{}}
	r := countryExemptRouter(t, mock, repo)

	body := bytes.NewBufferString(`{"countries":["IL","Israel"]}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/security/crowdsec/country-exemption", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Empty(t, mock.Calls(), "agent must not be called on pre-validation failure")
	assert.Empty(t, repo.getResult.CountryExemptCountries, "settings must not be persisted")
}

func TestCountryExempt_Put_AgentFailure_DoesNotPersist(t *testing.T) {

	mock := agent.NewMockClient().OnError("security.crowdsec.country_exempt.set", &agent.AgentError{
		Code: agent.CodeInternal, Message: "crowdsec reload failed",
	})
	repo := &mockServerSettingsRepo{getResult: &models.ServerSettings{}}
	r := countryExemptRouter(t, mock, repo)

	body := bytes.NewBufferString(`{"countries":["IL"]}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/security/crowdsec/country-exemption", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.GreaterOrEqual(t, rec.Code, 500)
	assert.Empty(t, repo.getResult.CountryExemptCountries,
		"agent-first ordering: a failed host write must not drift the DB")
}

func TestCountryExempt_Put_HappyPath(t *testing.T) {

	mock := agent.NewMockClient().On("security.crowdsec.country_exempt.set",
		map[string]any{"countries": []any{"IL", "US"}})
	repo := &mockServerSettingsRepo{getResult: &models.ServerSettings{}}
	r := countryExemptRouter(t, mock, repo)

	// Lowercase + duplicate codes must be normalized before the agent call.
	body := bytes.NewBufferString(`{"countries":["us","IL","il","US"]}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/security/crowdsec/country-exemption", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	calls := mock.Calls()
	require.Len(t, calls, 1)
	assert.Equal(t, "security.crowdsec.country_exempt.set", calls[0].Command)
	var params map[string][]string
	require.NoError(t, json.Unmarshal(calls[0].Params, &params))
	assert.ElementsMatch(t, []string{"IL", "US"}, params["countries"])

	assert.Equal(t, "US,IL", repo.getResult.CountryExemptCountries)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, []any{"US", "IL"}, resp["countries"])
}

func TestCountryExempt_Put_EmptyList_TurnsFeatureOff(t *testing.T) {

	mock := agent.NewMockClient().On("security.crowdsec.country_exempt.set",
		map[string]any{"countries": []any{}})
	repo := &mockServerSettingsRepo{getResult: &models.ServerSettings{CountryExemptCountries: "IL"}}
	r := countryExemptRouter(t, mock, repo)

	body := bytes.NewBufferString(`{"countries":[]}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/security/crowdsec/country-exemption", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, repo.getResult.CountryExemptCountries)
}

func TestCountryExempt_Sync_Returns202(t *testing.T) {

	repo := &mockServerSettingsRepo{getResult: &models.ServerSettings{CountryExemptCountries: "IL"}}
	r := countryExemptRouter(t, agent.NewMockClient(), repo)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/security/crowdsec/country-exemption/sync", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code)
}
