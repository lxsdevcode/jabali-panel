package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/auth"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/ginctx"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/models"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/repository"
)

type fakeGrants struct {
	grant *models.ImpersonationGrant
}

func (f *fakeGrants) Create(context.Context, *models.ImpersonationGrant) error { return nil }
func (f *fakeGrants) FindActive(_ context.Context, id string, _ time.Time) (*models.ImpersonationGrant, error) {
	if f.grant != nil && f.grant.ID == id {
		return f.grant, nil
	}
	return nil, repository.ErrNotFound
}
func (f *fakeGrants) End(context.Context, string, time.Time) error { return nil }
func (f *fakeGrants) ListActiveByAdmin(context.Context, string, time.Time) ([]models.ImpersonationGrant, error) {
	return nil, nil
}
func (f *fakeGrants) ReapExpired(context.Context, time.Time) (int64, error) { return 0, nil }

type fakeUsers struct{ users map[string]*models.User }

func (f *fakeUsers) FindByID(_ context.Context, id string) (*models.User, error) {
	if u, ok := f.users[id]; ok {
		return u, nil
	}
	return nil, repository.ErrNotFound
}

// run builds a one-route engine: a fake auth middleware sets `real`, then
// ResolveImpersonation runs, then the handler reports the EFFECTIVE claims.
func run(t *testing.T, real *auth.AccessClaims, grants *fakeGrants, users *fakeUsers, path, header string) (int, *auth.AccessClaims) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if real != nil {
			ginctx.SetClaims(c, real)
		}
		c.Next()
	})
	now := func() time.Time { return time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC) }
	r.Use(ResolveImpersonation(grants, users, now))
	var got *auth.AccessClaims
	r.GET("/*any", func(c *gin.Context) {
		got = ginctx.Claims(c)
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if header != "" {
		req.Header.Set(ImpersonateHeader, header)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code, got
}

func adminClaims() *auth.AccessClaims {
	return &auth.AccessClaims{UserID: "admin1", Email: "admin@x", IsAdmin: true}
}

func validGrant() *fakeGrants {
	return &fakeGrants{grant: &models.ImpersonationGrant{
		ID: "grant1", AdminUserID: "admin1", TargetUserID: "user9",
		ExpiresAt: time.Date(2026, 6, 15, 13, 0, 0, 0, time.UTC),
	}}
}

func usersWith(u ...*models.User) *fakeUsers {
	m := map[string]*models.User{}
	for _, x := range u {
		m[x.ID] = x
	}
	return &fakeUsers{users: m}
}

func TestResolveImpersonation_NoHeader_NoOverride(t *testing.T) {
	code, got := run(t, adminClaims(), validGrant(), usersWith(), "/api/v1/domains", "")
	if code != http.StatusOK || got == nil || got.UserID != "admin1" || got.ImpersonatedBy != "" {
		t.Fatalf("no header should not override: code=%d got=%+v", code, got)
	}
}

func TestResolveImpersonation_NonAdminHeader_Ignored(t *testing.T) {
	real := &auth.AccessClaims{UserID: "user9", IsAdmin: false}
	code, got := run(t, real, validGrant(), usersWith(), "/api/v1/domains", "grant1")
	if code != http.StatusOK || got.UserID != "user9" || got.ImpersonatedBy != "" {
		t.Fatalf("non-admin header must be ignored (no override, no 403): code=%d got=%+v", code, got)
	}
}

func TestResolveImpersonation_ValidGrant_Overrides(t *testing.T) {
	target := &models.User{ID: "user9", Email: "u9@x", IsAdmin: false}
	code, got := run(t, adminClaims(), validGrant(), usersWith(target), "/api/v1/domains", "grant1")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if got.UserID != "user9" || got.IsAdmin != false || got.ImpersonatedBy != "admin1" {
		t.Fatalf("override wrong: %+v", got)
	}
}

func TestResolveImpersonation_GrantForDifferentAdmin_403(t *testing.T) {
	g := validGrant()
	g.grant.AdminUserID = "someoneelse"
	code, _ := run(t, adminClaims(), g, usersWith(&models.User{ID: "user9"}), "/api/v1/domains", "grant1")
	if code != http.StatusForbidden {
		t.Fatalf("grant owned by another admin must 403, got %d", code)
	}
}

func TestResolveImpersonation_InvalidGrant_403(t *testing.T) {
	code, _ := run(t, adminClaims(), &fakeGrants{}, usersWith(), "/api/v1/domains", "nope")
	if code != http.StatusForbidden {
		t.Fatalf("invalid/expired grant must 403, got %d", code)
	}
}

func TestResolveImpersonation_TargetIsAdmin_403(t *testing.T) {
	target := &models.User{ID: "user9", IsAdmin: true}
	code, _ := run(t, adminClaims(), validGrant(), usersWith(target), "/api/v1/domains", "grant1")
	if code != http.StatusForbidden {
		t.Fatalf("must refuse to act as an admin target, got %d", code)
	}
}

func TestResolveImpersonation_ManagementPath_NoOverride(t *testing.T) {
	target := &models.User{ID: "user9", IsAdmin: false}
	// Even with a valid grant + header, the management path stays the real admin.
	code, got := run(t, adminClaims(), validGrant(), usersWith(target), "/api/v1/admin/impersonation/grant1", "grant1")
	if code != http.StatusOK || got.UserID != "admin1" || got.ImpersonatedBy != "" {
		t.Fatalf("management path must not be overridden: code=%d got=%+v", code, got)
	}
}
