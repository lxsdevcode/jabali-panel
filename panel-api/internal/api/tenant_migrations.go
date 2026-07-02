package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/agent"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/ginctx"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/repository"
)

// TenantMigrationsConfig wires the tenant-facing WordPress migration surface.
// EVERY endpoint is owner-scoped: a tenant may only migrate INTO a domain they
// own, target_user_id is always forced to the authenticated caller (never taken
// from the request), and only the WordPress source kinds are allowed (panel-
// account migrations stay admin-only).
type TenantMigrationsConfig struct {
	Jobs    repository.MigrationJobRepository
	Domains repository.DomainRepository
	Users   repository.UserRepository
	Agent   agent.AgentInterface
}

type tenantMigrationsHandler struct{ cfg TenantMigrationsConfig }

// RegisterTenantMigrationRoutes mounts /migrations/* for authenticated tenants.
func RegisterTenantMigrationRoutes(rg *gin.RouterGroup, cfg TenantMigrationsConfig) {
	if cfg.Jobs == nil || cfg.Domains == nil || cfg.Users == nil || cfg.Agent == nil {
		return
	}
	h := &tenantMigrationsHandler{cfg: cfg}
	g := rg.Group("/migrations")
	g.POST("/wordpress", h.create)
	g.POST("/:id/secrets", h.uploadSecrets)
	g.POST("/:id/pull-source", h.pull)
	g.POST("/:id/import-wp", h.importWP)
	g.GET("/:id", h.get)
}

// caller returns the authenticated user's id, or "" + writes 401.
func (h *tenantMigrationsHandler) caller(c *gin.Context) string {
	claims := ginctx.Claims(c)
	if claims == nil || claims.UserID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return ""
	}
	return claims.UserID
}

// ownedJob loads the :id job and confirms the caller owns it (target_user_id).
// A job the caller does not own returns 404 (not 403) so existence never leaks.
func (h *tenantMigrationsHandler) ownedJob(c *gin.Context, uid string) *models.MigrationJob {
	job, err := h.cfg.Jobs.FindByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return nil
	}
	if job.TargetUserID == nil || *job.TargetUserID != uid {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return nil
	}
	return job
}

// ownedDomain confirms the caller owns dest domain `name`; returns it or writes
// the error response.
func (h *tenantMigrationsHandler) ownedDomain(c *gin.Context, uid, name string) *models.Domain {
	dom, err := h.cfg.Domains.FindByName(c.Request.Context(), name)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "domain_not_found"})
			return nil
		}
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return nil
	}
	if dom.UserID != uid {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "domain_not_found"})
		return nil
	}
	return dom
}

type tenantCreateWPRequest struct {
	SourceKind string `json:"source_kind"`
	SourceHost string `json:"source_host"`
	SourceUser string `json:"source_user"`
	SourcePath string `json:"source_path"`
	DestDomain string `json:"dest_domain"`
}

// create — a tenant creates a WordPress migration INTO a domain they own.
func (h *tenantMigrationsHandler) create(c *gin.Context) {
	uid := h.caller(c)
	if uid == "" {
		return
	}
	var req tenantCreateWPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	// Only WordPress source kinds — tenants can't run panel-account migrations.
	if req.SourceKind != models.MigrationSourceWordPressSSH &&
		req.SourceKind != models.MigrationSourceWordPressPlugin {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported_kind", "detail": "tenant migrations support wordpress_ssh or wordpress_plugin"})
		return
	}
	if strings.TrimSpace(req.SourceHost) == "" || strings.TrimSpace(req.DestDomain) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing_fields", "detail": "source_host and dest_domain required"})
		return
	}
	// Dest must be a domain the caller owns.
	if h.ownedDomain(c, uid, req.DestDomain) == nil {
		return
	}
	// Caller's OS username = the forced import destination user.
	user, err := h.cfg.Users.FindByID(c.Request.Context(), uid)
	if err != nil || user.Username == nil || *user.Username == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user_lookup"})
		return
	}
	sourceUser := strings.TrimSpace(req.SourceUser)
	if sourceUser == "" {
		sourceUser = "wp" // unused for wordpress_plugin
	}
	forcedUID := uid // target is ALWAYS the caller — never from the request
	destUser := *user.Username
	destDomain := strings.TrimSpace(req.DestDomain)
	row := &models.MigrationJob{
		ID:           genULID(),
		SourceKind:   req.SourceKind,
		SourceHost:   strings.TrimSpace(req.SourceHost),
		SourceUser:   sourceUser,
		TargetUserID: &forcedUID,
		DestUser:     &destUser,   // set -> pull auto-imports (background job)
		DestDomain:   &destDomain,
		State:        models.MigrationStatePending,
		StartedAt:    time.Now().UTC(),
	}
	if err := h.cfg.Jobs.Create(c.Request.Context(), row); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal", "detail": err.Error()})
		return
	}
	if sp := strings.TrimSpace(req.SourcePath); sp != "" {
		_ = h.cfg.Jobs.UpdateSourcePath(c.Request.Context(), row.ID, sp)
	}
	c.JSON(http.StatusCreated, gin.H{"id": row.ID, "source_kind": row.SourceKind, "state": row.State})
}

type tenantSecretsRequest struct {
	SSHPassword   string `json:"ssh_password"`
	SSHPrivateKey string `json:"ssh_private_key"`
	PluginToken   string `json:"plugin_token"`
}

func (h *tenantMigrationsHandler) uploadSecrets(c *gin.Context) {
	uid := h.caller(c)
	if uid == "" {
		return
	}
	if h.ownedJob(c, uid) == nil {
		return
	}
	var req tenantSecretsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	if req.SSHPassword == "" && req.SSHPrivateKey == "" && req.PluginToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing_credential"})
		return
	}
	h.callAgent(c, "migration.secrets_write", map[string]any{
		"job_id":          c.Param("id"),
		"ssh_password":    req.SSHPassword,
		"ssh_private_key": req.SSHPrivateKey,
		"plugin_token":    req.PluginToken,
	})
}

type tenantPullRequest struct {
	SSHUser string `json:"ssh_user"`
}

func (h *tenantMigrationsHandler) pull(c *gin.Context) {
	uid := h.caller(c)
	if uid == "" {
		return
	}
	if h.ownedJob(c, uid) == nil {
		return
	}
	var req tenantPullRequest
	_ = c.ShouldBindJSON(&req)
	h.callAgent(c, "migration.pull_source_run", map[string]any{
		"job_id":   c.Param("id"),
		"ssh_user": req.SSHUser,
	})
}

type tenantImportRequest struct {
	DestDomain string `json:"dest_domain"`
}

// importWP — dest_user is FORCED to the caller's own OS username and the dest
// domain must be one the caller owns. Neither is taken on trust from the body.
func (h *tenantMigrationsHandler) importWP(c *gin.Context) {
	uid := h.caller(c)
	if uid == "" {
		return
	}
	if h.ownedJob(c, uid) == nil {
		return
	}
	var req tenantImportRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.DestDomain) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dest_domain required"})
		return
	}
	if h.ownedDomain(c, uid, req.DestDomain) == nil {
		return
	}
	user, err := h.cfg.Users.FindByID(c.Request.Context(), uid)
	if err != nil || user.Username == nil || *user.Username == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user_lookup"})
		return
	}
	h.callAgent(c, "migration.import_wp_run", map[string]any{
		"job_id":      c.Param("id"),
		"dest_user":   *user.Username, // FORCED — the caller's own OS user
		"dest_domain": req.DestDomain,
	})
}

func (h *tenantMigrationsHandler) get(c *gin.Context) {
	uid := h.caller(c)
	if uid == "" {
		return
	}
	job := h.ownedJob(c, uid)
	if job == nil {
		return
	}
	c.JSON(http.StatusOK, job)
}

// callAgent invokes an agent verb and maps the result to a JSON response.
func (h *tenantMigrationsHandler) callAgent(c *gin.Context, verb string, params map[string]any) {
	if h.cfg.Agent == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent_unconfigured"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if _, err := h.cfg.Agent.Call(ctx, verb, params); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "agent_error", "detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
