package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/auth"
	ginctx "git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/ginctx"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/models"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/repository"
)

// ownerFilterDomainRepo records which list path the handler took and serves
// owner-filtered data, so the test can assert the ?user_id branch (#483).
type ownerFilterDomainRepo struct {
	*mockDomainRepo
	calledList     bool
	calledByUserID string
}

func (r *ownerFilterDomainRepo) List(_ context.Context, _ repository.ListOptions) ([]models.Domain, int64, error) {
	r.calledList = true
	out := make([]models.Domain, 0, len(r.domains))
	for _, d := range r.domains {
		out = append(out, *d)
	}
	return out, int64(len(out)), nil
}

func (r *ownerFilterDomainRepo) ListByUserID(_ context.Context, userID string, _ repository.ListOptions) ([]models.Domain, int64, error) {
	r.calledByUserID = userID
	out := make([]models.Domain, 0)
	for _, d := range r.domains {
		if d.UserID == userID {
			out = append(out, *d)
		}
	}
	return out, int64(len(out)), nil
}

// TestDomainList_AdminUserIDFilter covers the admin owner-scope query param
// added for #483: a valid ?user_id routes through ListByUserID and filters to
// that owner; an invalid ULID is rejected 400; absent param lists all.
func TestDomainList_AdminUserIDFilter(t *testing.T) {
	const ownerA = "01ARZ3NDEKTSV4RRFFQ69G5FAV" // canonical valid ULID
	const ownerB = "01BX5ZZKBKACTAV9WEVGEMMVRZ"

	newRouter := func(repo repository.DomainRepository) *gin.Engine {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		v1 := r.Group("/api/v1")
		v1.Use(func(c *gin.Context) {
			ginctx.SetClaims(c, &auth.AccessClaims{UserID: "admin1", IsAdmin: true})
			c.Next()
		})
		RegisterDomainRoutes(v1, DomainHandlerConfig{Domains: repo})
		return r
	}

	seed := func() *ownerFilterDomainRepo {
		base := newMockDomainRepo()
		base.Create(context.Background(), &models.Domain{ID: "d1", UserID: ownerA, Name: "a.com"})
		base.Create(context.Background(), &models.Domain{ID: "d2", UserID: ownerA, Name: "b.com"})
		base.Create(context.Background(), &models.Domain{ID: "d3", UserID: ownerB, Name: "c.com"})
		return &ownerFilterDomainRepo{mockDomainRepo: base}
	}

	t.Run("valid user_id filters to that owner via ListByUserID", func(t *testing.T) {
		repo := seed()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/domains?user_id="+ownerA, nil)
		newRouter(repo).ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status: got %d want 200, body=%s", w.Code, w.Body.String())
		}
		if repo.calledByUserID != ownerA {
			t.Errorf("expected ListByUserID(%q), got %q (calledList=%v)", ownerA, repo.calledByUserID, repo.calledList)
		}
		var resp struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
			Total int `json:"total"`
		}
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp.Total != 2 || len(resp.Data) != 2 {
			t.Errorf("expected 2 domains for ownerA, got total=%d len=%d", resp.Total, len(resp.Data))
		}
	})

	t.Run("invalid user_id is rejected 400", func(t *testing.T) {
		repo := seed()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/domains?user_id=not-a-ulid", nil)
		newRouter(repo).ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status: got %d want 400, body=%s", w.Code, w.Body.String())
		}
		if repo.calledByUserID != "" || repo.calledList {
			t.Errorf("no repo list call expected on validation failure")
		}
	})

	t.Run("absent user_id lists all via List", func(t *testing.T) {
		repo := seed()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/domains", nil)
		newRouter(repo).ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status: got %d want 200", w.Code)
		}
		if !repo.calledList || repo.calledByUserID != "" {
			t.Errorf("expected List path, got calledList=%v calledByUserID=%q", repo.calledList, repo.calledByUserID)
		}
	})
}
