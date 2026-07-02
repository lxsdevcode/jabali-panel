package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/auth"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/ginctx"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
	"github.com/gin-gonic/gin"
)

// GH #307: tenant-safe nginx options are owner-settable ONLY when the admin
// opts in (server_settings.tenant_domain_options_enabled). Admins always may.
func buildSafeOptionsRouter(claims *auth.AccessClaims, toggle bool, dom *models.Domain) (*gin.Engine, *mockDomainRepo) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.Use(func(c *gin.Context) { ginctx.SetClaims(c, claims); c.Next() })
	repo := newMockDomainRepo()
	repo.domains[dom.ID] = dom
	settings := &mockServerSettingsRepo{getResult: &models.ServerSettings{TenantDomainOptionsEnabled: toggle}}
	RegisterDomainRoutes(v1, DomainHandlerConfig{Domains: repo, ServerSettings: settings})
	return r, repo
}

func patchSafeOptions(t *testing.T, r *gin.Engine) int {
	t.Helper()
	body := bytes.NewBufferString(`{"nginx_safe_options":{"max_body_mb":128,"hsts":true}}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/domains/d1", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w.Code
}

func TestDomainPatch_SafeOptions_OwnerGatedByToggle(t *testing.T) {
	t.Run("owner with opt-in OFF: dropped", func(t *testing.T) {
		dom := &models.Domain{ID: "d1", UserID: "u1", Name: "x.com"}
		r, repo := buildSafeOptionsRouter(&auth.AccessClaims{UserID: "u1"}, false, dom)
		if code := patchSafeOptions(t, r); code != http.StatusOK {
			t.Fatalf("status %d", code)
		}
		if repo.domains["d1"].NginxSafeOptions.MaxBodyMB != 0 {
			t.Errorf("owner set safe options while opt-in OFF: %+v", repo.domains["d1"].NginxSafeOptions)
		}
	})

	t.Run("owner with opt-in ON: applied", func(t *testing.T) {
		dom := &models.Domain{ID: "d1", UserID: "u1", Name: "x.com"}
		r, repo := buildSafeOptionsRouter(&auth.AccessClaims{UserID: "u1"}, true, dom)
		if code := patchSafeOptions(t, r); code != http.StatusOK {
			t.Fatalf("status %d", code)
		}
		got := repo.domains["d1"].NginxSafeOptions
		if got.MaxBodyMB != 128 || !got.HSTS {
			t.Errorf("owner safe options not applied with opt-in ON: %+v", got)
		}
	})

	t.Run("admin: applied regardless of toggle", func(t *testing.T) {
		dom := &models.Domain{ID: "d1", UserID: "u1", Name: "x.com"}
		r, repo := buildSafeOptionsRouter(&auth.AccessClaims{UserID: "admin", IsAdmin: true}, false, dom)
		if code := patchSafeOptions(t, r); code != http.StatusOK {
			t.Fatalf("status %d", code)
		}
		if repo.domains["d1"].NginxSafeOptions.MaxBodyMB != 128 {
			t.Errorf("admin safe options not applied: %+v", repo.domains["d1"].NginxSafeOptions)
		}
	})
}
