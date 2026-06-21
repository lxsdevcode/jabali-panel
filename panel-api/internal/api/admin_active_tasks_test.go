package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/agent"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/api"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/models"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/repository"
)

type atUpdateRepo struct {
	repository.UpdateHistoryRepository
	rows []models.UpdateHistory
}

func (m *atUpdateRepo) ListRunning(context.Context) ([]models.UpdateHistory, error) {
	return m.rows, nil
}

type atBackupRepo struct {
	repository.BackupJobRepository
	running []models.BackupJob
}

func (m *atBackupRepo) ListByStatusSince(_ context.Context, status string, _ time.Time, _ int) ([]models.BackupJob, error) {
	if status == models.BackupJobStatusRunning {
		return m.running, nil
	}
	return nil, nil
}

func TestAdminActiveTasks_Aggregates(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now()
	updates := &atUpdateRepo{rows: []models.UpdateHistory{
		{Kind: models.UpdateKindJabali, Action: models.UpdateActionRun, Status: models.UpdateStatusRunning, StartedAt: now},
		{Kind: models.UpdateKindApt, Action: models.UpdateActionRun, Status: models.UpdateStatusRunning, StartedAt: now},
		{Kind: models.UpdateKindJabali, Action: models.UpdateActionCheck, Status: models.UpdateStatusRunning, StartedAt: now}, // skipped
	}}
	backups := &atBackupRepo{running: []models.BackupJob{
		{Kind: models.BackupJobKindSystemBackup, Status: models.BackupJobStatusRunning, CreatedAt: now},
	}}
	ag := agent.NewMockClient().On("security.malware.active_scans", map[string]any{"active": 1})

	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.Use(injectAdminClaims(true))
	api.RegisterAdminActiveTasksRoutes(v1, api.AdminActiveTasksConfig{Updates: updates, BackupJobs: backups, Agent: ag})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/active-tasks", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Count int `json:"count"`
		Tasks []struct {
			Kind, Label, Status string
		} `json:"tasks"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	// 2 update runs (check skipped) + 1 backup + 1 malware = 4
	if resp.Count != 4 {
		t.Fatalf("want 4 tasks, got %d: %s", resp.Count, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"Panel update", "System packages update", "System backup", "Malware scan"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing task %q in %s", want, body)
		}
	}
}

func TestAdminActiveTasks_RBAC(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.Use(injectAdminClaims(false))
	api.RegisterAdminActiveTasksRoutes(v1, api.AdminActiveTasksConfig{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/active-tasks", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin want 403, got %d", rec.Code)
	}
}
