// admin_active_tasks.go — GET /admin/active-tasks: a unified view of the
// long-running operations in flight (panel/system-package updates, backups,
// malware scans), for the top-bar Tasks indicator. Read-only, admin-only,
// best-effort: a missing/erroring source contributes nothing rather than
// failing the whole list.
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/agent"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/middleware"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/models"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/repository"
)

// AdminActiveTasksConfig bundles the task sources.
type AdminActiveTasksConfig struct {
	Updates    repository.UpdateHistoryRepository
	BackupJobs repository.BackupJobRepository
	Agent      agent.AgentInterface
}

type activeTask struct {
	Kind      string `json:"kind"` // "update" | "backup" | "malware"
	Label     string `json:"label"`
	Status    string `json:"status"` // "running" | "queued"
	StartedAt string `json:"started_at,omitempty"`
}

// RegisterAdminActiveTasksRoutes mounts GET /admin/active-tasks (RequireAdmin).
func RegisterAdminActiveTasksRoutes(g *gin.RouterGroup, cfg AdminActiveTasksConfig) {
	h := &adminActiveTasksHandler{cfg: cfg}
	grp := g.Group("/admin/active-tasks")
	grp.Use(middleware.RequireAdmin())
	grp.GET("", h.get)
}

type adminActiveTasksHandler struct{ cfg AdminActiveTasksConfig }

func (h *adminActiveTasksHandler) get(c *gin.Context) {
	ctx := c.Request.Context()
	tasks := make([]activeTask, 0)

	// Updates — running panel (jabali) + system-packages (apt) runs.
	if h.cfg.Updates != nil {
		if rows, err := h.cfg.Updates.ListRunning(ctx); err == nil {
			for _, r := range rows {
				if r.Action != models.UpdateActionRun {
					continue // a fast "check" isn't a task worth surfacing
				}
				label := "Panel update"
				if r.Kind == models.UpdateKindApt {
					label = "System packages update"
				}
				tasks = append(tasks, activeTask{
					Kind: "update", Label: label, Status: "running",
					StartedAt: r.StartedAt.UTC().Format(time.RFC3339),
				})
			}
		}
	}

	// Backups — running + queued in the last 24h.
	if h.cfg.BackupJobs != nil {
		since := time.Now().Add(-24 * time.Hour)
		for _, st := range []string{models.BackupJobStatusRunning, models.BackupJobStatusQueued} {
			jobs, err := h.cfg.BackupJobs.ListByStatusSince(ctx, st, since, 20)
			if err != nil {
				continue
			}
			for i := range jobs {
				tasks = append(tasks, activeTask{
					Kind: "backup", Label: backupTaskLabel(jobs[i].Kind), Status: st,
					StartedAt: jobs[i].CreatedAt.UTC().Format(time.RFC3339),
				})
			}
		}
	}

	// Malware — active maldet scans (transient units), via the agent.
	if h.cfg.Agent != nil {
		callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		raw, err := h.cfg.Agent.Call(callCtx, "security.malware.active_scans", nil)
		cancel()
		if err == nil {
			var v struct {
				Active int `json:"active"`
			}
			if json.Unmarshal(raw, &v) == nil {
				for i := 0; i < v.Active; i++ {
					tasks = append(tasks, activeTask{Kind: "malware", Label: "Malware scan", Status: "running"})
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"tasks": tasks, "count": len(tasks)})
}

func backupTaskLabel(kind string) string {
	switch kind {
	case models.BackupJobKindAccountBackup:
		return "Account backup"
	case models.BackupJobKindAccountRestore:
		return "Account restore"
	case models.BackupJobKindSystemBackup:
		return "System backup"
	case models.BackupJobKindSystemRestore:
		return "System restore"
	default:
		return "Backup"
	}
}
