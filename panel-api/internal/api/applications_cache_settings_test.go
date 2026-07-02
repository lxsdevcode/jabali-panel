package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
)

// Exercises GET/PUT /applications/:id/cache-settings end-to-end through the
// real handler + router (GH #612/#616/#618). Confirms the JSON envelope the
// SPA drawer codes against, and the GET->PUT->GET round-trip persistence +
// unconfigured→configured transition.
func seedCacheInstall(t *testing.T) (*mockWordPressInstallRepo, *mockDomainRepo, *mockUserRepo, string) {
	t.Helper()
	wpRepo, domRepo, userRepo := wpUserAndDomain()
	id := "01INSTALLCACHE0000000000AA"
	if err := wpRepo.Create(context.Background(), &models.WordPressInstall{
		ID: id, UserID: "user1", DomainID: "domain1", AppType: "wordpress",
		Subdirectory: "", Status: "ready", CacheEnabled: true,
	}); err != nil {
		t.Fatalf("seed install: %v", err)
	}
	return wpRepo, domRepo, userRepo, id
}

func TestCacheSettings_RoundTrip(t *testing.T) {
	wpRepo, domRepo, userRepo, id := seedCacheInstall(t)
	r, _, _ := applicationsRouter(t, "user1", false, wpRepo, domRepo, userRepo, nil)

	// GET on a never-configured install → configured=false, empty settings.
	get := func() map[string]any {
		req := httptest.NewRequest("GET", "/api/v1/applications/"+id+"/cache-settings", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("GET status %d: %s", w.Code, w.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("GET decode: %v", err)
		}
		return body
	}

	b := get()
	if b["cache_enabled"] != true {
		t.Errorf("cache_enabled = %v, want true", b["cache_enabled"])
	}
	if b["configured"] != false {
		t.Errorf("configured = %v, want false (never configured)", b["configured"])
	}

	// PUT a woocommerce profile with page cache on, a TTL, and an exclusion.
	pageOn := true
	ttl := 600
	put := models.CacheSettingsData{
		Profile:       "woocommerce",
		PageCache:     &pageOn,
		TTLSeconds:    &ttl,
		URLExclusions: []string{"/private"},
	}
	buf, _ := json.Marshal(put)
	req := httptest.NewRequest("PUT", "/api/v1/applications/"+id+"/cache-settings", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT status %d: %s", w.Code, w.Body.String())
	}

	// GET again → configured=true and the values round-trip.
	b = get()
	if b["configured"] != true {
		t.Fatalf("configured = %v after PUT, want true", b["configured"])
	}
	settings, _ := b["settings"].(map[string]any)
	if settings["profile"] != "woocommerce" {
		t.Errorf("profile = %v, want woocommerce", settings["profile"])
	}
	if settings["page_cache"] != true {
		t.Errorf("page_cache = %v, want true", settings["page_cache"])
	}
	if settings["ttl_seconds"].(float64) != 600 {
		t.Errorf("ttl_seconds = %v, want 600", settings["ttl_seconds"])
	}
	ex, _ := settings["url_exclusions"].([]any)
	if len(ex) != 1 || ex[0] != "/private" {
		t.Errorf("url_exclusions = %v, want [/private]", settings["url_exclusions"])
	}
}

func TestCacheSettings_Validation(t *testing.T) {
	wpRepo, domRepo, userRepo, id := seedCacheInstall(t)
	r, _, _ := applicationsRouter(t, "user1", false, wpRepo, domRepo, userRepo, nil)

	put := func(bodyJSON string) int {
		req := httptest.NewRequest("PUT", "/api/v1/applications/"+id+"/cache-settings", bytes.NewReader([]byte(bodyJSON)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}

	if code := put(`{"profile":"not-a-real-profile"}`); code != http.StatusBadRequest {
		t.Errorf("bad profile → %d, want 400", code)
	}
	if code := put(`{"ttl_seconds":5}`); code != http.StatusBadRequest {
		t.Errorf("ttl below min → %d, want 400", code)
	}
	if code := put(`{"ttl_seconds":999999}`); code != http.StatusBadRequest {
		t.Errorf("ttl above max → %d, want 400", code)
	}
	if code := put(`{"profile":"membership"}`); code != http.StatusOK {
		t.Errorf("valid membership profile → %d, want 200", code)
	}
}

// A different user must not read another's install settings (owner scope).
func TestCacheSettings_OwnerScope(t *testing.T) {
	wpRepo, domRepo, userRepo, id := seedCacheInstall(t)
	r, _, _ := applicationsRouter(t, "intruder", false, wpRepo, domRepo, userRepo, nil)
	req := httptest.NewRequest("GET", "/api/v1/applications/"+id+"/cache-settings", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("cross-tenant GET → %d, want 404", w.Code)
	}
}
