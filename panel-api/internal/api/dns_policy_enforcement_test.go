package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/ids"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedZone provisions a domain + zone for the given owner so record handlers
// can run.
func seedZone(t *testing.T, domainRepo *mockDomainRepo, zoneRepo *mockDNSZoneRepo, owner string) {
	t.Helper()
	domainRepo.Create(context.Background(), &models.Domain{ID: "test-domain-id", UserID: owner, Name: "example.com"})
	zoneRepo.Create(context.Background(), &models.DNSZone{
		ID: ids.NewULID(), DomainID: "test-domain-id", Name: "example.com",
		Serial: 1, RefreshSeconds: 3600, RetrySeconds: 600, ExpireSeconds: 604800,
		MinimumTTL: 3600, IsEnabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})
}

func postRecord(t *testing.T, r http.Handler, typ, content string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"name": "x", "type": typ, "content": content, "ttl": 3600})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/domains/test-domain-id/dns/records", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestDNSRecordPolicyEnforcement covers GH #466: with a locked-down policy a
// non-admin tenant may create A but not MX, while an admin bypasses the matrix.
func TestDNSRecordPolicyEnforcement(t *testing.T) {
	locked, _ := models.DNSPolicyPreset("locked-down")
	settings := &models.ServerSettings{ID: 1, DNSUserRecordPolicy: locked}

	t.Run("non-admin denied disallowed type", func(t *testing.T) {
		r, dr, zr := dnsRouterWithSettings("user1", false, settings)
		seedZone(t, dr, zr, "user1")
		w := postRecord(t, r, "MX", "10 mail.example.com.")
		require.Equal(t, http.StatusForbidden, w.Code)
		var resp map[string]any
		json.NewDecoder(w.Body).Decode(&resp)
		assert.Equal(t, "record_type_forbidden", resp["error"])
	})

	t.Run("non-admin allowed permitted type", func(t *testing.T) {
		r, dr, zr := dnsRouterWithSettings("user1", false, settings)
		seedZone(t, dr, zr, "user1")
		w := postRecord(t, r, "A", "192.0.2.1")
		require.Equal(t, http.StatusCreated, w.Code)
	})

	t.Run("admin bypasses the matrix", func(t *testing.T) {
		r, dr, zr := dnsRouterWithSettings("admin1", true, settings)
		seedZone(t, dr, zr, "user1") // admin acts on someone else's domain
		w := postRecord(t, r, "MX", "10 mail.example.com.")
		require.Equal(t, http.StatusCreated, w.Code)
	})

	t.Run("GET /dns/policy reflects the matrix for users", func(t *testing.T) {
		r, _, _ := dnsRouterWithSettings("user1", false, settings)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/dns/policy", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
		var resp struct {
			Policy  models.DNSUserRecordPolicy `json:"policy"`
			IsAdmin bool                       `json:"is_admin"`
		}
		json.NewDecoder(w.Body).Decode(&resp)
		assert.False(t, resp.IsAdmin)
		assert.True(t, resp.Policy.Allows("A", "create"))
		assert.False(t, resp.Policy.Allows("MX", "create"))
	})
}
