package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/auth"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/ginctx"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/models"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/repository"
)

type duUserRepo struct {
	repository.UserRepository
	u *models.User
}

func (r *duUserRepo) FindByID(context.Context, string) (*models.User, error) { return r.u, nil }

type duDomainRepo struct {
	repository.DomainRepository
	doms []models.Domain
}

func (r *duDomainRepo) ListByUserID(context.Context, string, repository.ListOptions) ([]models.Domain, int64, error) {
	return r.doms, int64(len(r.doms)), nil
}

type duMailboxRepo struct {
	repository.MailboxRepository
	mbs []models.Mailbox
}

func (r *duMailboxRepo) ListByDomainID(context.Context, string, repository.ListOptions) ([]models.Mailbox, int64, error) {
	return r.mbs, int64(len(r.mbs)), nil
}

type duDatabaseRepo struct {
	repository.DatabaseRepository
	dbs []models.Database
}

func (r *duDatabaseRepo) ListByUserID(context.Context, string, repository.ListOptions) ([]models.Database, int64, error) {
	return r.dbs, int64(len(r.dbs)), nil
}

type duAgent struct{}

func (duAgent) Call(_ context.Context, method string, _ any) (json.RawMessage, error) {
	switch method {
	case "user.limits.report":
		return json.Marshal(map[string]any{"disk": map[string]uint64{"used_kb": 1024, "limit_kb": 10240}})
	case "db.size":
		return json.Marshal(map[string]int64{"size_bytes": 2048})
	}
	return nil, fmt.Errorf("unexpected agent call %q", method)
}

func TestMeDiskUsage_Aggregates(t *testing.T) {
	gin.SetMode(gin.TestMode)
	uname := "alice"
	cfg := DiskUsageConfig{
		Users:      &duUserRepo{u: &models.User{ID: "u1", Username: &uname}},
		Domains:    &duDomainRepo{doms: []models.Domain{{ID: "d1"}}},
		Mailboxes:  &duMailboxRepo{mbs: []models.Mailbox{{EmailCached: "a@x", LastUsageBytes: 3000}, {EmailCached: "b@x", LastUsageBytes: 500}}},
		Databases:  &duDatabaseRepo{dbs: []models.Database{{Name: "db1", Engine: "mariadb"}}},
		Agent:      duAgent{},
		QuotaMount: "/home",
	}

	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.Use(func(c *gin.Context) {
		ginctx.SetClaims(c, &auth.AccessClaims{UserID: "u1"})
		c.Next()
	})
	RegisterMeDiskUsageRoutes(v1, cfg)

	// GET before any refresh returns an empty snapshot (computed_at nil) — the
	// page never auto-computes on entry.
	greq := httptest.NewRequest(http.MethodGet, "/api/v1/me/disk-usage", nil)
	grec := httptest.NewRecorder()
	r.ServeHTTP(grec, greq)
	var empty diskUsageResponse
	_ = json.Unmarshal(grec.Body.Bytes(), &empty)
	if empty.ComputedAt != nil || empty.TotalBytes != 0 {
		t.Errorf("GET before refresh must be empty, got total=%d computed_at=%v", empty.TotalBytes, empty.ComputedAt)
	}

	// POST refresh recomputes live (where the aggregation now lives).
	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/disk-usage/refresh", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d %s", rec.Code, rec.Body.String())
	}

	var resp diskUsageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ComputedAt == nil {
		t.Error("refresh must set computed_at")
	}
	if resp.Files.Bytes != 1024*1024 {
		t.Errorf("files: want %d, got %d", 1024*1024, resp.Files.Bytes)
	}
	if resp.QuotaBytes != 10240*1024 {
		t.Errorf("quota: want %d, got %d", 10240*1024, resp.QuotaBytes)
	}
	if resp.Email.Bytes != 3500 || len(resp.Email.Items) != 2 {
		t.Errorf("email: want 3500/2, got %d/%d", resp.Email.Bytes, len(resp.Email.Items))
	}
	if resp.Databases.Bytes != 2048 || len(resp.Databases.Items) != 1 {
		t.Errorf("databases: want 2048/1, got %d/%d", resp.Databases.Bytes, len(resp.Databases.Items))
	}
	want := uint64(1024*1024 + 3500 + 2048)
	if resp.TotalBytes != want {
		t.Errorf("total: want %d, got %d", want, resp.TotalBytes)
	}
}
