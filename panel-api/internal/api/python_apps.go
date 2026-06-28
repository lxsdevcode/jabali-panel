package api

import (
	"context"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/oklog/ulid/v2"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/agent"
	ginctx "git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/ginctx"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/ids"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/models"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/repository"
)

// PythonAppHandlerConfig wires the Python Application Manager endpoints
// (ADR-0131 / GH #203).
type PythonAppHandlerConfig struct {
	Apps       repository.PythonAppRepository
	Domains    repository.DomainRepository
	Settings   repository.ServerSettingsRepository
	Agent      agent.AgentInterface
	Reconciler interface{ Schedule(id string) }
}

type pythonAppHandler struct{ cfg PythonAppHandlerConfig }

// RegisterPythonAppRoutes mounts /python-apps (authenticated; owner-scoped).
// Every route is gated on the python_apps_enabled server setting.
func RegisterPythonAppRoutes(g *gin.RouterGroup, cfg PythonAppHandlerConfig) {
	h := &pythonAppHandler{cfg: cfg}
	grp := g.Group("/python-apps")
	grp.Use(h.requireEnabled)
	grp.GET("", h.list)
	grp.POST("", h.create)
	grp.GET("/:id", h.get)
	grp.DELETE("/:id", h.delete)
	grp.POST("/:id/control", h.control)
	grp.GET("/:id/logs", h.logs)
	grp.PUT("/:id/env", h.putEnv)
}

func (h *pythonAppHandler) requireEnabled(c *gin.Context) {
	if h.cfg.Settings != nil {
		if s, err := h.cfg.Settings.Get(c.Request.Context()); err == nil && s != nil && s.PythonAppsEnabled {
			c.Next()
			return
		}
	}
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "python_apps_disabled"})
}

var (
	baseURIRe    = regexp.MustCompile(`^/[A-Za-z0-9._/-]*$`)
	pyVersionRe  = regexp.MustCompile(`^3\.(?:[0-9]|1[0-9])$`)
	entrypointRe = regexp.MustCompile(`^[A-Za-z0-9_.]+:[A-Za-z0-9_]+$`)
)

// loadOwned fetches the app and enforces owner/admin scope.
func (h *pythonAppHandler) loadOwned(c *gin.Context) (*models.PythonApp, bool) {
	claims := ginctx.Claims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return nil, false
	}
	app, err := h.cfg.Apps.FindByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return nil, false
	}
	if !claims.IsAdmin && app.UserID != claims.UserID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return nil, false
	}
	return app, true
}

func (h *pythonAppHandler) list(c *gin.Context) {
	claims := ginctx.Claims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return
	}
	var apps []*models.PythonApp
	var err error
	if claims.IsAdmin {
		// Admin owner-scope via ?user_id (#483).
		if uid := c.Query("user_id"); uid != "" {
			if !ids.IsValidULID(uid) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
				return
			}
			apps, err = h.cfg.Apps.ListByUser(c.Request.Context(), uid)
		} else {
			apps, err = h.cfg.Apps.ListAll(c.Request.Context())
		}
	} else {
		apps, err = h.cfg.Apps.ListByUser(c.Request.Context(), claims.UserID)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": apps})
}

func (h *pythonAppHandler) get(c *gin.Context) {
	app, ok := h.loadOwned(c)
	if !ok {
		return
	}
	env, _ := h.cfg.Apps.ListEnv(c.Request.Context(), app.ID)
	c.JSON(http.StatusOK, gin.H{"app": app, "env": env})
}

type createPythonAppRequest struct {
	DomainID      string            `json:"domain_id" binding:"required"`
	Name          string            `json:"name" binding:"required"`
	PythonVersion string            `json:"python_version" binding:"required"`
	AppRoot       string            `json:"app_root" binding:"required"`
	AppType       string            `json:"app_type"`
	Entrypoint    string            `json:"entrypoint" binding:"required"`
	BaseURI       string            `json:"base_uri"`
	Env           map[string]string `json:"env,omitempty"`
}

func (h *pythonAppHandler) create(c *gin.Context) {
	claims := ginctx.Claims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return
	}
	var req createPythonAppRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "detail": err.Error()})
		return
	}
	if req.AppType == "" {
		req.AppType = "wsgi"
	}
	if req.BaseURI == "" {
		req.BaseURI = "/"
	}
	if !pyVersionRe.MatchString(req.PythonVersion) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_python_version"})
		return
	}
	if req.AppType != "wsgi" && req.AppType != "asgi" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_app_type"})
		return
	}
	if !entrypointRe.MatchString(req.Entrypoint) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_entrypoint", "detail": "expected module:callable"})
		return
	}
	if !baseURIRe.MatchString(req.BaseURI) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_base_uri"})
		return
	}

	ctx := c.Request.Context()
	domain, err := h.cfg.Domains.FindByID(ctx, req.DomainID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "domain_not_found"})
		return
	}
	if !claims.IsAdmin && domain.UserID != claims.UserID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	port, err := h.cfg.Apps.FindFreeLoopbackPort(ctx)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no_free_port"})
		return
	}

	app := &models.PythonApp{
		ID:            ulid.Make().String(),
		UserID:        domain.UserID,
		DomainID:      domain.ID,
		Name:          req.Name,
		Runtime:       "python",
		PythonVersion: req.PythonVersion,
		AppRoot:       req.AppRoot,
		AppType:       req.AppType,
		Entrypoint:    req.Entrypoint,
		BaseURI:       req.BaseURI,
		LoopbackPort:  &port,
		Status:        models.PythonAppStatusPending,
	}
	if err := h.cfg.Apps.Create(ctx, app); err != nil {
		if isConflict(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "mount_in_use", "detail": "another app already owns this domain + path"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}
	if len(req.Env) > 0 {
		_ = h.cfg.Apps.ReplaceEnv(ctx, app.ID, envMapToRows(req.Env))
	}

	// Attach the nginx proxy to the domain (reuses the proxy_pass rule path).
	h.attachProxyRule(ctx, domain, app)
	if h.cfg.Reconciler != nil {
		h.cfg.Reconciler.Schedule(domain.ID)
	}
	c.JSON(http.StatusCreated, gin.H{"app": app})
}

func (h *pythonAppHandler) delete(c *gin.Context) {
	app, ok := h.loadOwned(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	if h.cfg.Agent != nil {
		_, _ = h.cfg.Agent.Call(ctx, "app.python.remove", map[string]any{"app_id": app.ID})
	}
	// Detach the proxy rule from the domain.
	if domain, err := h.cfg.Domains.FindByID(ctx, app.DomainID); err == nil {
		h.detachProxyRule(ctx, domain, app)
		if h.cfg.Reconciler != nil {
			h.cfg.Reconciler.Schedule(domain.ID)
		}
	}
	if err := h.cfg.Apps.Delete(ctx, app.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (h *pythonAppHandler) control(c *gin.Context) {
	app, ok := h.loadOwned(c)
	if !ok {
		return
	}
	var req struct {
		Action string `json:"action" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	raw, err := h.cfg.Agent.Call(c.Request.Context(), "app.python.control", map[string]any{"app_id": app.ID, "action": req.Action})
	if err != nil {
		respondAgentError(c, err)
		return
	}
	c.Data(http.StatusOK, "application/json", raw)
}

func (h *pythonAppHandler) logs(c *gin.Context) {
	app, ok := h.loadOwned(c)
	if !ok {
		return
	}
	raw, err := h.cfg.Agent.Call(c.Request.Context(), "app.python.logs", map[string]any{"app_id": app.ID, "lines": 200})
	if err != nil {
		respondAgentError(c, err)
		return
	}
	c.Data(http.StatusOK, "application/json", raw)
}

func (h *pythonAppHandler) putEnv(c *gin.Context) {
	app, ok := h.loadOwned(c)
	if !ok {
		return
	}
	var req struct {
		Env map[string]string `json:"env"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	if err := h.cfg.Apps.ReplaceEnv(c.Request.Context(), app.ID, envMapToRows(req.Env)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}
	if h.cfg.Reconciler != nil {
		h.cfg.Reconciler.Schedule(app.DomainID)
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// attachProxyRule appends a proxy_pass nginx rule (base_uri -> loopback port)
// to the domain so the existing reconciler renders the vhost.
func (h *pythonAppHandler) attachProxyRule(ctx context.Context, domain *models.Domain, app *models.PythonApp) {
	target := proxyTargetFor(app)
	ws := true
	rule := models.NginxRule{Type: "proxy_pass", Path: app.BaseURI, Target: target, Websocket: &ws}
	rules := append(models.NginxRules{}, domain.NginxRules...)
	// Replace any existing rule for the same path, else append.
	replaced := false
	for i := range rules {
		if rules[i].Type == "proxy_pass" && rules[i].Path == app.BaseURI {
			rules[i] = rule
			replaced = true
			break
		}
	}
	if !replaced {
		rules = append(rules, rule)
	}
	domain.NginxRules = rules
	_ = h.cfg.Domains.Update(ctx, domain)
}

func (h *pythonAppHandler) detachProxyRule(ctx context.Context, domain *models.Domain, app *models.PythonApp) {
	target := proxyTargetFor(app)
	var kept models.NginxRules
	for _, r := range domain.NginxRules {
		if r.Type == "proxy_pass" && r.Target == target {
			continue
		}
		kept = append(kept, r)
	}
	domain.NginxRules = kept
	_ = h.cfg.Domains.Update(ctx, domain)
}

func proxyTargetFor(app *models.PythonApp) string {
	port := 0
	if app.LoopbackPort != nil {
		port = *app.LoopbackPort
	}
	return "http://127.0.0.1:" + itoaPort(port)
}

func itoaPort(p int) string {
	return strings.TrimSpace(intToStr(p))
}

func envMapToRows(m map[string]string) []models.PythonAppEnv {
	rows := make([]models.PythonAppEnv, 0, len(m))
	for k, v := range m {
		rows = append(rows, models.PythonAppEnv{ID: ulid.Make().String(), Key: k, Value: v})
	}
	return rows
}
