package api

// GH #1005: a Postgres database reported 0 B under Disk Usage → Databases.
//
// #1012 fixed the Databases page by teaching the agent's db.size to read
// pg_database_size() when it is handed engine="postgres". This page kept
// two bugs that made that fix invisible here:
//
//  1. it gated the size call on engine == "mariadb", so Postgres rows were
//     never sized at all, and
//  2. dbSize did not forward the engine, so even an ungated call would have
//     been answered from MariaDB's information_schema — which has no row for
//     a Postgres database and sums to 0 with NO error. The wrong query looks
//     exactly like an empty database.
//
// Both are pinned here: the engine has to reach the agent, and both engines
// have to be sized.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/auth"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/ginctx"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
)

// duEngineAgent answers db.size per engine and records what it was asked,
// so the test can assert the engine actually crossed the wire.
type duEngineAgent struct {
	mu    sync.Mutex
	seen  map[string]string // db_name -> engine received
	sizes map[string]int64  // engine -> size to report
}

func (a *duEngineAgent) Call(_ context.Context, method string, params any) (json.RawMessage, error) {
	switch method {
	case "user.limits.report":
		return json.Marshal(map[string]any{"disk": map[string]uint64{"used_kb": 0, "limit_kb": 10240}})
	case "db.size":
		m, _ := params.(map[string]any)
		name, _ := m["db_name"].(string)
		engine, _ := m["engine"].(string)
		a.mu.Lock()
		if a.seen == nil {
			a.seen = map[string]string{}
		}
		a.seen[name] = engine
		a.mu.Unlock()
		// Mirror the real agent: a Postgres database asked for with the
		// MariaDB query sums to 0 rather than erroring.
		if engine == "postgres" {
			return json.Marshal(map[string]int64{"size_bytes": a.sizes["postgres"]})
		}
		return json.Marshal(map[string]int64{"size_bytes": a.sizes["mariadb"]})
	}
	return nil, fmt.Errorf("unexpected agent call %q", method)
}

func (a *duEngineAgent) engineFor(db string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.seen[db]
}

func TestMeDiskUsage_SizesPostgresDatabases(t *testing.T) {
	gin.SetMode(gin.TestMode)
	uname := "alice"
	ag := &duEngineAgent{sizes: map[string]int64{"mariadb": 2048, "postgres": 4096}}
	cfg := DiskUsageConfig{
		Users:     &duUserRepo{u: &models.User{ID: "u1", Username: &uname}},
		Domains:   &duDomainRepo{},
		Mailboxes: &duMailboxRepo{},
		Databases: &duDatabaseRepo{dbs: []models.Database{
			{Name: "alice_shop", Engine: "mariadb"},
			{Name: "alice_pg", Engine: "postgres"},
		}},
		Agent:      ag,
		QuotaMount: "/home",
	}

	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.Use(func(c *gin.Context) {
		ginctx.SetClaims(c, &auth.AccessClaims{UserID: "u1"})
		c.Next()
	})
	RegisterMeDiskUsageRoutes(v1, cfg)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/me/disk-usage/refresh", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("refresh returned %d: %s", rec.Code, rec.Body.String())
	}
	var got diskUsageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}

	// The engine must reach the agent, or db.size answers from
	// information_schema and returns a silent 0.
	if e := ag.engineFor("alice_pg"); e != "postgres" {
		t.Errorf("db.size for the Postgres database was called with engine=%q, want \"postgres\" — "+
			"without it the agent sums MariaDB's information_schema and reports 0 B", e)
	}
	if e := ag.engineFor("alice_shop"); e != "mariadb" {
		t.Errorf("db.size for the MariaDB database was called with engine=%q, want \"mariadb\"", e)
	}

	byName := map[string]uint64{}
	for _, it := range got.Databases.Items {
		byName[it.Name] = it.Bytes
	}
	if byName["alice_pg"] != 4096 {
		t.Errorf("Postgres database reported %d B, want 4096 — this is the 0 B the issue reported", byName["alice_pg"])
	}
	if byName["alice_shop"] != 2048 {
		t.Errorf("MariaDB database reported %d B, want 2048 (regression in the path that already worked)", byName["alice_shop"])
	}
	if got.Databases.Bytes != 6144 {
		t.Errorf("databases total = %d, want 6144 — the Postgres row must count toward the total too", got.Databases.Bytes)
	}
}
