package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestEnforceSameOriginForCookieMutations covers the global CSRF guard (GH #460):
// cross-origin cookie mutations are rejected; same-origin, safe-method, and
// Bearer-token requests pass.
func TestEnforceSameOriginForCookieMutations(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const host = "panel.example.com:8443"

	cases := []struct {
		name       string
		method     string
		origin     string
		referer    string
		authHeader string
		wantStatus int
	}{
		{"safe GET cross-origin allowed", http.MethodGet, "https://evil.example/", "", "", http.StatusOK},
		{"same-origin POST allowed", http.MethodPost, "https://panel.example.com:8443", "", "", http.StatusOK},
		{"same-origin POST diff port allowed", http.MethodPost, "https://panel.example.com", "", "", http.StatusOK},
		{"cross-origin POST rejected", http.MethodPost, "https://evil.example", "", "", http.StatusForbidden},
		{"cross-origin PUT rejected", http.MethodPut, "https://evil.example", "", "", http.StatusForbidden},
		{"cross-origin DELETE rejected", http.MethodDelete, "https://evil.example", "", "", http.StatusForbidden},
		{"bearer token exempt even cross-origin", http.MethodPost, "https://evil.example", "", "Bearer abc123", http.StatusOK},
		{"no origin no referer rejected", http.MethodPost, "", "", "", http.StatusForbidden},
		{"origin absent referer same-origin allowed", http.MethodPost, "", "https://panel.example.com:8443/some/page", "", http.StatusOK},
		{"origin absent referer cross-origin rejected", http.MethodPost, "", "https://evil.example/x", "", http.StatusForbidden},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			ctx, engine := gin.CreateTestContext(w)
			engine.Use(EnforceSameOriginForCookieMutations())
			engine.Handle(c.method, "/api/v1/x", func(g *gin.Context) {
				g.Status(http.StatusOK)
			})

			req := httptest.NewRequest(c.method, "/api/v1/x", nil)
			req.Host = host
			if c.origin != "" {
				req.Header.Set("Origin", c.origin)
			}
			if c.referer != "" {
				req.Header.Set("Referer", c.referer)
			}
			if c.authHeader != "" {
				req.Header.Set("Authorization", c.authHeader)
			}
			ctx.Request = req
			engine.ServeHTTP(w, req)

			if w.Code != c.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, c.wantStatus)
			}
		})
	}
}
