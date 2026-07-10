package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"git.jabali-panel.com/shukivaknin/jabali2/internal/kratosclient"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/api"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/auth"
	ginctx "git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/ginctx"
)

// JAB-3 regression guard: DELETE /admin/sessions/:id (panel session revoke) must
// flush the positive whoami cache so the revoked session can't be accepted for
// the remaining TTL. Proven behaviorally: a cached whoami is served without an
// upstream hit, then the revoke forces the next Whoami to re-validate.
func TestRevokeSession_FlushesWhoamiCache(t *testing.T) {
	var whoamiHits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/sessions/whoami":
			atomic.AddInt32(&whoamiHits, 1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"sess-1","active":true,"identity":{"id":"idA"}}`))
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/admin/sessions/"):
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	kc := kratosclient.NewClient(srv.URL, srv.URL)
	ctx := context.Background()

	// Prime + confirm the cache serves the second call without a round-trip.
	_, _ = kc.Whoami(ctx, "cookieA")
	_, _ = kc.Whoami(ctx, "cookieA")
	if got := atomic.LoadInt32(&whoamiHits); got != 1 {
		t.Fatalf("whoami hits before revoke = %d; want 1 (cached)", got)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/api/v1", func(c *gin.Context) {
		ginctx.SetClaims(c, &auth.AccessClaims{UserID: "admin-1", IsAdmin: true})
		c.Next()
	})
	api.RegisterUserRoutes(g, api.UserHandlerConfig{
		Repo:         newMemUserRepo(),
		KratosClient: kc,
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/sessions/sess-1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Cache was flushed → the next Whoami re-validates upstream.
	_, _ = kc.Whoami(ctx, "cookieA")
	if got := atomic.LoadInt32(&whoamiHits); got != 2 {
		t.Fatalf("whoami hits after revoke = %d; want 2 (cache flushed, refetched)", got)
	}
}
