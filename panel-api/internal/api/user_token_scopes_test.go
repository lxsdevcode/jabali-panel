package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/models"
)

// ctxKey mirrors the unexported middleware key the auth layer stashes the
// token under (middleware.UserAPIToken reads it).
const userTokenCtxKey = "jabali_user_api_token"

func scopeTestRouter(tok *models.UserAPIToken) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1", func(c *gin.Context) {
		if tok != nil {
			c.Set(userTokenCtxKey, tok)
		}
		c.Next()
	}, EnforceUserTokenScopes())
	ok := func(c *gin.Context) { c.Status(http.StatusOK) }
	// A couple of mapped DNS routes + one unmapped (mail) route.
	v1.GET("/domains/:id/dns/records", ok)
	v1.POST("/domains/:id/dns/records", ok)
	v1.PATCH("/dns/records/:recordId", ok)
	v1.GET("/domains/:id/mail/mailboxes", ok) // unmapped
	return r
}

func reqStatus(r *gin.Engine, method, path string) int {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	r.ServeHTTP(w, req)
	return w.Code
}

func TestEnforceUserTokenScopes(t *testing.T) {
	full := &models.UserAPIToken{Scopes: models.UserAPIScopes{}} // empty = full
	readDNS := &models.UserAPIToken{Scopes: models.UserAPIScopes{ScopeReadDNS}}
	writeDNS := &models.UserAPIToken{Scopes: models.UserAPIScopes{ScopeWriteDNS}}
	ddns := &models.UserAPIToken{Scopes: models.UserAPIScopes{ScopeDDNS}}

	cases := []struct {
		name   string
		tok    *models.UserAPIToken
		method string
		path   string
		want   int
	}{
		{"cookie auth (no token) allowed", nil, "GET", "/api/v1/domains/d1/dns/records", http.StatusOK},
		{"full token any route", full, "GET", "/api/v1/domains/d1/mail/mailboxes", http.StatusOK},
		{"read:dns reads dns", readDNS, "GET", "/api/v1/domains/d1/dns/records", http.StatusOK},
		{"read:dns cannot write dns", readDNS, "POST", "/api/v1/domains/d1/dns/records", http.StatusForbidden},
		{"write:dns writes dns", writeDNS, "POST", "/api/v1/domains/d1/dns/records", http.StatusOK},
		{"write:dns on record route", writeDNS, "PATCH", "/api/v1/dns/records/r1", http.StatusOK},
		{"dns token denied on unmapped mail", writeDNS, "GET", "/api/v1/domains/d1/mail/mailboxes", http.StatusForbidden},
		{"ddns token denied on dns REST", ddns, "GET", "/api/v1/domains/d1/dns/records", http.StatusForbidden},
		{"ddns token denied on unmapped", ddns, "GET", "/api/v1/domains/d1/mail/mailboxes", http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := scopeTestRouter(tc.tok)
			if got := reqStatus(r, tc.method, tc.path); got != tc.want {
				t.Errorf("%s %s: got %d want %d", tc.method, tc.path, got, tc.want)
			}
		})
	}
}

func TestValidateUserScopes(t *testing.T) {
	if _, ok := validateUserScopes(models.UserAPIScopes{ScopeReadDNS, ScopeWriteDNS, ScopeDDNS}); !ok {
		t.Error("known scopes should validate")
	}
	if _, ok := validateUserScopes(models.UserAPIScopes{}); !ok {
		t.Error("empty scopes (full) should validate")
	}
	if bad, ok := validateUserScopes(models.UserAPIScopes{"write:everything"}); ok || bad != "write:everything" {
		t.Errorf("unknown scope should fail, got bad=%q ok=%v", bad, ok)
	}
}
