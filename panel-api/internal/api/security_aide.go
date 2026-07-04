package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/agent"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/middleware"
)

// M42 (ADR-0087) AIDE FIM admin routes. Read-only status + manual
// check trigger. Filesystem (/var/lib/aide/aide.db) is the truth;
// DB doesn't mirror state.

const aideCallTimeout = 10 * time.Second
const aideCheckTimeout = 11 * time.Minute

func RegisterSecurityAideRoutes(rg *gin.RouterGroup, cli agent.AgentInterface) {
	g := rg.Group("/admin/security/aide", middleware.RequireAdmin())

	g.GET("/status", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), aideCallTimeout)
		defer cancel()
		raw, err := cli.Call(ctx, "security.aide.status", map[string]any{})
		if err != nil {
			status, body := translateAgentError(err)
			c.JSON(status, body)
			return
		}
		c.Data(http.StatusOK, "application/json; charset=utf-8", raw)
	})

	// GH #714: full-diff export — the raw AIDE report for download.
	g.GET("/report", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), aideCallTimeout)
		defer cancel()
		raw, err := cli.Call(ctx, "security.aide.report", map[string]any{})
		if err != nil {
			status, body := translateAgentError(err)
			c.JSON(status, body)
			return
		}
		c.Data(http.StatusOK, "application/json; charset=utf-8", raw)
	})

	g.POST("/check", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), aideCheckTimeout)
		defer cancel()
		raw, err := cli.Call(ctx, "security.aide.check", map[string]any{})
		if err != nil {
			status, body := translateAgentError(err)
			c.JSON(status, body)
			return
		}
		c.Data(http.StatusOK, "application/json; charset=utf-8", raw)
	})

	// Gitea #561: rebuild the AIDE baseline (aideinit -y -f). dry_run returns
	// the plan only. Destructive — replaces the integrity baseline.
	g.POST("/rebuild", func(c *gin.Context) {
		var body struct {
			DryRun  bool   `json:"dry_run"`
			Confirm string `json:"confirm"`
		}
		_ = c.ShouldBindJSON(&body)
		// GH #683: a real rebuild replaces the integrity baseline with the CURRENT
		// filesystem state (aideinit -y -f) — if the host is already compromised
		// this blesses the tampered files. Require an explicit typed confirmation
		// and record an audit entry; dry_run (plan only) stays unguarded.
		if !body.DryRun {
			if body.Confirm != "REBUILD" {
				c.JSON(http.StatusBadRequest, gin.H{
					"status": "error", "error": "confirmation_required",
					"detail": `set "confirm":"REBUILD" to replace the AIDE baseline`,
				})
				return
			}
			c.Set("audit_target", "aide integrity baseline")
			c.Set("audit_target_type", "security_aide_rebuild")
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 16*time.Minute)
		defer cancel()
		raw, err := cli.Call(ctx, "security.aide.rebuild", map[string]any{"dry_run": body.DryRun})
		if err != nil {
			status, ebody := translateAgentError(err)
			c.JSON(status, ebody)
			return
		}
		c.Data(http.StatusOK, "application/json; charset=utf-8", raw)
	})
}
