// docker_apps.go — REST surface for the M48 Docker App Marketplace
// (admin-only per ADR-0116 Decision 1).
//
// Routes:
//
//   GET    /admin/docker-apps/catalog              list catalog entries
//   GET    /admin/docker-apps                      list installed apps
//   POST   /admin/docker-apps                      install a catalog app
//   GET    /admin/docker-apps/:id                  get one (with ports)
//   PATCH  /admin/docker-apps/:id                  update install-time settings
//   DELETE /admin/docker-apps/:id                  uninstall (?keep_volumes=1)
//   POST   /admin/docker-apps/:id/start            start
//   POST   /admin/docker-apps/:id/stop             stop
//   POST   /admin/docker-apps/:id/restart          restart
//   POST   /admin/docker-apps/:id/rebuild          force-recreate
//
// Phase 4 ships READ + CRUD + lifecycle. Logs, exec, backup,
// update/rollback, compose-download all queue for later phases.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oklog/ulid/v2"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/agent"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/dockerapp"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/ginctx"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/models"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/repository"
)

// DockerAppHandlerConfig bundles dependencies.
type DockerAppHandlerConfig struct {
	Repo    repository.DockerAppRepository
	Catalog *dockerapp.Catalog
	// ServerSettings is the gate for the M48 marketplace opt-in. When
	// settings.docker_marketplace_enabled is false, every route in
	// this group returns 503 docker_marketplace_disabled. The flag is
	// flipped on the Server Settings page; the panel-api server_settings
	// PATCH handler dispatches docker.install / docker.disable to the
	// agent on flip.
	ServerSettings repository.ServerSettingsRepository
	// Domains is optional. When set, the install handler creates a
	// `domains` row with managed_by='docker_app' for each install
	// that has a loopback+reverse_proxy=true port, so the reconciler
	// renders an nginx vhost that proxy_passes to the upstream port
	// AND issues LE through the same path as tenant domains.
	// ADR-0116 Decision 4.
	Domains repository.DomainRepository
	Agent   agent.AgentInterface
	Log     *slog.Logger
}

type dockerAppHandler struct{ cfg DockerAppHandlerConfig }

// RegisterDockerAppRoutes mounts the admin docker-app surface.
func RegisterDockerAppRoutes(g *gin.RouterGroup, cfg DockerAppHandlerConfig) {
	h := &dockerAppHandler{cfg: cfg}
	grp := g.Group("/admin/docker-apps")
	grp.Use(requireAdminForDockerApps)
	grp.Use(h.requireDockerMarketplaceEnabled)
	grp.GET("/catalog", h.listCatalog)
	grp.GET("", h.list)
	grp.POST("", h.install)
	grp.GET("/:id", h.get)
	grp.PATCH("/:id", h.update)
	grp.DELETE("/:id", h.delete)
	grp.POST("/:id/start", h.lifecycle("start"))
	grp.POST("/:id/stop", h.lifecycle("stop"))
	grp.POST("/:id/restart", h.lifecycle("restart"))
	grp.POST("/:id/rebuild", h.lifecycle("rebuild"))
	grp.POST("/:id/update", h.updateImage)
}

// requireAdminForDockerApps gates the whole route group on
// claims.IsAdmin. Tenants can never reach any verb on this surface
// (ADR-0116 Decision 1).
func requireAdminForDockerApps(c *gin.Context) {
	claims := ginctx.Claims(c)
	if claims == nil || !claims.IsAdmin {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	c.Next()
}

// ---- catalog -----------------------------------------------------------------

type catalogEntryResponse struct {
	Slug          string                   `json:"slug"`
	Name          string                   `json:"name"`
	Version       string                   `json:"version"`
	Description   string                   `json:"description"`
	Icon          string                   `json:"icon"`
	Upstream      string                   `json:"upstream,omitempty"`
	Documentation string                   `json:"documentation,omitempty"`
	UpdateMode    string                   `json:"update_mode"`
	Resources     dockerapp.Resources      `json:"resources"`
	Volumes       []dockerapp.Volume       `json:"volumes"`
	Ports         []dockerapp.PortSpec     `json:"ports"`
	Env           []catalogEnvVarResponse  `json:"env,omitempty"`
}

type catalogEnvVarResponse struct {
	Name     string `json:"name"`
	Value    string `json:"value,omitempty"`     // omitted when Secret=true
	Secret   bool   `json:"secret,omitempty"`
	Generate string `json:"generate,omitempty"`
}

func (h *dockerAppHandler) listCatalog(c *gin.Context) {
	if h.cfg.Catalog == nil {
		c.JSON(http.StatusOK, gin.H{"items": []catalogEntryResponse{}})
		return
	}
	entries := h.cfg.Catalog.All()
	out := make([]catalogEntryResponse, 0, len(entries))
	for _, e := range entries {
		out = append(out, catalogEntryToResponse(e))
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}

func catalogEntryToResponse(e dockerapp.Entry) catalogEntryResponse {
	env := make([]catalogEnvVarResponse, 0, len(e.Env))
	for _, v := range e.Env {
		row := catalogEnvVarResponse{Name: v.Name, Secret: v.Secret, Generate: v.Generate}
		if !v.Secret {
			row.Value = v.Value
		}
		env = append(env, row)
	}
	return catalogEntryResponse{
		Slug:          e.Slug,
		Name:          e.Name,
		Version:       e.Version,
		Description:   e.Description,
		Icon:          e.Icon,
		Upstream:      e.Upstream,
		Documentation: e.Documentation,
		UpdateMode:    e.UpdateMode,
		Resources:     e.Resources,
		Volumes:       e.Volumes,
		Ports:         e.Ports,
		Env:           env,
	}
}

// ---- list installed ---------------------------------------------------------

type installedResponse struct {
	models.DockerApp
	Ports []*models.DockerAppPublishedPort `json:"ports"`
}

func (h *dockerAppHandler) list(c *gin.Context) {
	ctx := c.Request.Context()
	apps, err := h.cfg.Repo.ListAll(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}
	out := make([]installedResponse, 0, len(apps))
	for _, a := range apps {
		ports, _ := h.cfg.Repo.ListPortsForApp(ctx, a.ID)
		out = append(out, installedResponse{DockerApp: *a, Ports: ports})
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}

// ---- install ----------------------------------------------------------------

type installRequest struct {
	Slug       string                    `json:"slug" binding:"required"`
	Name       string                    `json:"name" binding:"required"`
	Domain     string                    `json:"domain,omitempty"`
	UpdateMode string                    `json:"update_mode,omitempty"`
	CPULimit   string                    `json:"cpu_limit,omitempty"`
	Memory     string                    `json:"memory_limit,omitempty"`
	PIDsLimit  *int                      `json:"pids_limit,omitempty"`
	EnvOverride map[string]string        `json:"env,omitempty"`
	Ports      []installPortRequest      `json:"ports,omitempty"`
}

type installPortRequest struct {
	Name          string `json:"name"`           // catalog port name
	Enabled       *bool  `json:"enabled"`        // nil = use catalog default
	BindInterface string `json:"bind_interface"` // "loopback" or "public:<managed_ip_id>"
	HostPort      int    `json:"host_port"`      // 0 = auto-allocate
	ReverseProxy  *bool  `json:"reverse_proxy"`  // nil = use catalog default
}

var nameRE = regexp.MustCompile(`^[a-z0-9-]{1,32}$`)

func (h *dockerAppHandler) install(c *gin.Context) {
	ctx := c.Request.Context()
	var req installRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "detail": err.Error()})
		return
	}

	// Validate catalog slug.
	if h.cfg.Catalog == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "catalog_unavailable"})
		return
	}
	entry, ok := h.cfg.Catalog.Get(req.Slug)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown_slug", "slug": req.Slug})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if !nameRE.MatchString(req.Name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_name", "detail": "must match ^[a-z0-9-]{1,32}$"})
		return
	}

	// Reject duplicate slug+name.
	if existing, _ := h.cfg.Repo.FindBySlugName(ctx, req.Slug, req.Name); existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "already_installed", "id": existing.ID})
		return
	}

	// Apply defaults from the catalog when caller omits.
	updateMode := req.UpdateMode
	if updateMode == "" {
		updateMode = entry.UpdateMode
	}
	if updateMode != models.DockerAppUpdateModeManual && updateMode != models.DockerAppUpdateModeAuto {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_update_mode"})
		return
	}
	cpu := req.CPULimit
	if cpu == "" {
		cpu = entry.Resources.CPU
	}
	mem := req.Memory
	if mem == "" {
		mem = entry.Resources.Memory
	}
	pids := req.PIDsLimit
	if pids == nil && entry.Resources.PIDs > 0 {
		p := entry.Resources.PIDs
		pids = &p
	}

	// Persist the docker_apps row first; downstream port allocation
	// references its ID. Status starts at `pending` so the reconciler
	// will dispatch the install if our synchronous dispatch below
	// fails for any reason.
	app := &models.DockerApp{
		ID:             ulid.Make().String(),
		Slug:           req.Slug,
		Name:           req.Name,
		CatalogVersion: entry.Version,
		Status:         models.DockerAppStatusPending,
		UpdateMode:     updateMode,
	}
	if cpu != "" {
		app.CPULimit = &cpu
	}
	if mem != "" {
		app.MemoryLimit = &mem
	}
	if pids != nil {
		app.PIDsLimit = pids
	}
	if err := h.cfg.Repo.Create(ctx, app); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "persist_failed", "detail": err.Error()})
		return
	}

	// Resolve the ports the caller asked for, defaulting to the
	// catalog's `default_enabled` set.
	resolvedPorts, runtimePorts, err := h.resolvePorts(ctx, entry, req.Ports, app.ID)
	if err != nil {
		_ = h.cfg.Repo.Delete(ctx, app.ID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "port_resolution_failed", "detail": err.Error()})
		return
	}
	for _, p := range resolvedPorts {
		if err := h.cfg.Repo.CreatePort(ctx, p); err != nil {
			_ = h.cfg.Repo.Delete(ctx, app.ID)
			c.JSON(http.StatusConflict, gin.H{"error": "port_persist_failed", "detail": err.Error()})
			return
		}
	}

	// M48 Phase 6: auto-create a domains row when the install supplies
	// a hostname AND there's at least one loopback+reverse_proxy=true
	// port. The reconciler picks the row up on its next tick and
	// renders an nginx vhost via the existing proxy_pass nginx_rules
	// path -- same flow as tenant Rule Builder proxy_pass entries.
	// We tag the row with managed_by='docker_app' so the tenant /domains
	// CRUD handler refuses to delete it (delete is owned by the
	// docker-app handler below).
	if h.cfg.Domains != nil && req.Domain != "" {
		claims := ginctx.Claims(c)
		var loopbackPort *models.DockerAppPublishedPort
		for _, p := range resolvedPorts {
			if p.BindInterface == "loopback" && p.ReverseProxy && p.Protocol == "tcp" {
				loopbackPort = p
				break
			}
		}
		if loopbackPort != nil && claims != nil {
			target := "http://127.0.0.1:" + intToStr(loopbackPort.HostPort)
			truePtr := true
			rules := models.NginxRules{{
				Type:      "proxy_pass",
				Path:      "/",
				Target:    target,
				Websocket: &truePtr,
			}}
			dom := &models.Domain{
				ID:          ulid.Make().String(),
				UserID:      claims.UserID,
				Name:        req.Domain,
				DocRoot:     "",
				IsEnabled:   true,
				NginxRules:  rules,
				ManagedBy:   models.DomainManagedByDockerApp,
				DockerAppID: &app.ID,
			}
			if derr := h.cfg.Domains.Create(ctx, dom); derr != nil {
				// Non-fatal: surface in last_error so the operator can
				// see it failed and either pick a different domain or
				// retry. We leave the app row in place; the operator
				// can delete it from the UI if they want a clean slate.
				msg := "domain auto-create failed: " + firstLineString(derr.Error())
				_ = h.cfg.Repo.UpdateStatus(ctx, app.ID, models.DockerAppStatusFailed, &msg)
				c.JSON(http.StatusConflict, gin.H{"error": "domain_create_failed", "detail": derr.Error(), "id": app.ID})
				return
			}
		}
	}

	// Materialise env (catalog + secrets + operator overrides).
	envMap, err := dockerapp.MaterialiseEnv(entry, req.EnvOverride)
	if err != nil {
		_ = h.cfg.Repo.Delete(ctx, app.ID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "env_materialise_failed", "detail": err.Error()})
		return
	}

	// Render the compose template.
	dataRoot := "/var/lib/jabali/docker-apps/" + entry.Slug
	composeYML, err := dockerapp.Render(entry, dockerapp.RenderParams{
		Slug:         entry.Slug,
		Name:         req.Name,
		Domain:       req.Domain,
		ImageChannel: entry.ImageChannel,
		DataRoot:     dataRoot,
		CPULimit:     cpu,
		MemoryLimit:  mem,
		Ports:        runtimePorts,
		Env:          envMap,
	})
	if err != nil {
		_ = h.cfg.Repo.Delete(ctx, app.ID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "render_failed", "detail": err.Error()})
		return
	}

	// Dispatch install via the agent (synchronous, with a sensible
	// deadline). On failure we surface the error AND leave the row in
	// `failed` state so the operator can retry from the UI.
	if h.cfg.Agent != nil {
		callCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
		defer cancel()
		_ = h.cfg.Repo.UpdateStatus(callCtx, app.ID, models.DockerAppStatusInstalling, nil)
		volumeNames := make([]string, 0, len(entry.Volumes))
		for _, v := range entry.Volumes {
			volumeNames = append(volumeNames, v.Name)
		}
		envFile := buildEnvFile(envMap)
		_, agentErr := h.cfg.Agent.Call(callCtx, "docker_app.install", map[string]any{
			"slug":                         entry.Slug,
			"compose_yml":                  composeYML,
			"env_file":                     envFile,
			"volumes":                      volumeNames,
			"wait_healthy":                 true,
			"healthcheck_timeout_seconds":  120,
		})
		if agentErr != nil {
			msg := firstLineString(agentErr.Error())
			_ = h.cfg.Repo.UpdateStatus(ctx, app.ID, models.DockerAppStatusFailed, &msg)
			c.JSON(http.StatusBadGateway, gin.H{"error": "agent_install_failed", "detail": msg, "id": app.ID})
			return
		}
		_ = h.cfg.Repo.UpdateStatus(ctx, app.ID, models.DockerAppStatusRunning, nil)
	}

	// Return the installed row.
	fresh, _ := h.cfg.Repo.FindByID(ctx, app.ID)
	ports, _ := h.cfg.Repo.ListPortsForApp(ctx, app.ID)
	if fresh != nil {
		c.JSON(http.StatusCreated, installedResponse{DockerApp: *fresh, Ports: ports})
		return
	}
	c.JSON(http.StatusCreated, installedResponse{DockerApp: *app, Ports: ports})
}

// resolvePorts walks the catalog's declared ports and produces
// (DB rows, runtime map) for the install request. Catalog defaults
// apply when the request omits a port row.
func (h *dockerAppHandler) resolvePorts(
	ctx context.Context,
	entry dockerapp.Entry,
	req []installPortRequest,
	appID string,
) ([]*models.DockerAppPublishedPort, map[string]dockerapp.RuntimePort, error) {
	overrideByName := make(map[string]installPortRequest, len(req))
	for _, r := range req {
		overrideByName[r.Name] = r
	}

	rows := make([]*models.DockerAppPublishedPort, 0)
	runtime := make(map[string]dockerapp.RuntimePort, len(entry.Ports))

	for _, p := range entry.Ports {
		o, hasOverride := overrideByName[p.Name]
		enabled := p.DefaultEnabled
		if hasOverride && o.Enabled != nil {
			enabled = *o.Enabled
		}
		if !enabled {
			continue
		}
		bind := p.DefaultBind
		if hasOverride && o.BindInterface != "" {
			bind = o.BindInterface
		}
		rp := p.DefaultReverseProxy
		if hasOverride && o.ReverseProxy != nil {
			rp = *o.ReverseProxy
		}
		// Allocate or pin host port.
		hostPort := 0
		if hasOverride && o.HostPort > 0 {
			hostPort = o.HostPort
		} else {
			free, err := h.cfg.Repo.FindFreeHostPort(ctx, bindInterfaceForDB(bind), p.Protocol)
			if err != nil {
				return nil, nil, err
			}
			hostPort = free
		}
		row := &models.DockerAppPublishedPort{
			ID:            ulid.Make().String(),
			AppID:         appID,
			PortName:      p.Name,
			ContainerPort: p.ContainerPort,
			BindInterface: bindInterfaceForDB(bind),
			HostPort:      hostPort,
			Protocol:      p.Protocol,
			ReverseProxy:  rp,
			Enabled:       true,
		}
		rows = append(rows, row)
		runtime[p.Name] = dockerapp.RuntimePort{
			HostPort:      hostPort,
			ContainerPort: p.ContainerPort,
			BindInterface: bindInterfaceForRuntime(bind),
			Protocol:      p.Protocol,
		}
	}
	return rows, runtime, nil
}

// bindInterfaceForDB normalises the input shape ("loopback" or
// "public" or "public:<ip_id>") for the DB column. We keep the form
// the operator chose so a future migration can resolve "public" to
// the specific managed-IP at agent-call time.
func bindInterfaceForDB(s string) string {
	if s == "" {
		return "loopback"
	}
	return s
}

// bindInterfaceForRuntime turns the DB-form into the actual IP
// literal that goes into the compose `ports:` mapping. Phase 4
// supports "loopback" -> 127.0.0.1. "public" without a specific
// managed-IP picked yet binds to 0.0.0.0; managed-IP resolution
// lands in Phase 6 when domains.docker_app_id is wired.
func bindInterfaceForRuntime(s string) string {
	switch {
	case s == "loopback" || s == "":
		return "127.0.0.1"
	case s == "public":
		return "0.0.0.0"
	case strings.HasPrefix(s, "public:"):
		// TODO(phase6): resolve <managed_ip_id> against managed_ips
		// repo and return the address.
		return "0.0.0.0"
	}
	return "127.0.0.1"
}

// buildEnvFile renders an env map into a docker-compose .env file
// shape. Secret values get NO additional escaping; values must be
// alphanumeric/safe — catalog-declared secrets use base64url and
// operator overrides are validated by the schema.
func buildEnvFile(env map[string]string) string {
	if len(env) == 0 {
		return ""
	}
	var b strings.Builder
	for k, v := range env {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(v)
		b.WriteByte('\n')
	}
	return b.String()
}

// ---- get / update / delete ---------------------------------------------------

func (h *dockerAppHandler) get(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")
	app, err := h.cfg.Repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}
	ports, _ := h.cfg.Repo.ListPortsForApp(ctx, id)
	c.JSON(http.StatusOK, installedResponse{DockerApp: *app, Ports: ports})
}

type updateRequest struct {
	UpdateMode  *string `json:"update_mode,omitempty"`
	CPULimit    *string `json:"cpu_limit,omitempty"`
	MemoryLimit *string `json:"memory_limit,omitempty"`
	PIDsLimit   *int    `json:"pids_limit,omitempty"`
}

func (h *dockerAppHandler) update(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")
	app, err := h.cfg.Repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}
	var req updateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "detail": err.Error()})
		return
	}
	if req.UpdateMode != nil {
		v := *req.UpdateMode
		if v != models.DockerAppUpdateModeManual && v != models.DockerAppUpdateModeAuto {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_update_mode"})
			return
		}
		app.UpdateMode = v
	}
	if req.CPULimit != nil {
		app.CPULimit = req.CPULimit
	}
	if req.MemoryLimit != nil {
		app.MemoryLimit = req.MemoryLimit
	}
	if req.PIDsLimit != nil {
		app.PIDsLimit = req.PIDsLimit
	}
	if err := h.cfg.Repo.Update(ctx, app); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "persist_failed"})
		return
	}
	c.JSON(http.StatusOK, app)
}

func (h *dockerAppHandler) delete(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")
	app, err := h.cfg.Repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}
	purge := c.Query("keep_volumes") == ""
	if h.cfg.Agent != nil {
		callCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
		defer cancel()
		if _, err := h.cfg.Agent.Call(callCtx, "docker_app.delete", map[string]any{
			"slug":          app.Slug,
			"purge_volumes": purge,
		}); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "agent_delete_failed", "detail": err.Error()})
			return
		}
	}

	// M48 Phase 6: drop the auto-managed domain row (if any). The FK
	// on docker_app_id is SET NULL, not CASCADE -- we ON_DELETE_SET_NULL
	// at the SQL layer so an accidental docker_apps.Delete doesn't tear
	// down domain rows blindly; the explicit cleanup belongs here.
	if h.cfg.Domains != nil {
		domList, _, _ := h.cfg.Domains.List(ctx, repository.ListOptions{})
		for _, dom := range domList {
			if dom.ManagedBy == models.DomainManagedByDockerApp && dom.DockerAppID != nil && *dom.DockerAppID == id {
				_ = h.cfg.Domains.Delete(ctx, dom.ID)
			}
		}
	}

	if err := h.cfg.Repo.Delete(ctx, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "persist_failed"})
		return
	}
	c.Status(http.StatusNoContent)
}

// ---- lifecycle ---------------------------------------------------------------

func (h *dockerAppHandler) lifecycle(action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		id := c.Param("id")
		app, err := h.cfg.Repo.FindByID(ctx, id)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
			return
		}
		verb := "docker_app." + action
		if h.cfg.Agent == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent_unavailable"})
			return
		}
		callCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		raw, err := h.cfg.Agent.Call(callCtx, verb, map[string]any{"slug": app.Slug})
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "agent_call_failed", "detail": err.Error()})
			return
		}
		c.Data(http.StatusOK, "application/json; charset=utf-8", raw)
	}
}

// updateImage handles POST /:id/update. Dispatches docker_app.update
// and applies the new status to the row based on the agent's outcome.
func (h *dockerAppHandler) updateImage(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")
	app, err := h.cfg.Repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}
	if h.cfg.Agent == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent_unavailable"})
		return
	}

	// Mark `updating` so the UI shows a spinner; restore on outcome.
	_ = h.cfg.Repo.UpdateStatus(ctx, app.ID, models.DockerAppStatusUpdating, nil)

	callCtx, cancel := context.WithTimeout(ctx, 6*time.Minute)
	defer cancel()
	raw, err := h.cfg.Agent.Call(callCtx, "docker_app.update", map[string]any{
		"slug":                        app.Slug,
		"healthcheck_timeout_seconds": 180,
	})
	if err != nil {
		msg := firstLineString(err.Error())
		_ = h.cfg.Repo.UpdateStatus(ctx, app.ID, models.DockerAppStatusFailed, &msg)
		c.JSON(http.StatusBadGateway, gin.H{"error": "agent_update_failed", "detail": msg})
		return
	}
	// Parse the outcome to set the right terminal status.
	var resp struct {
		Outcome string `json:"outcome"`
		Detail  string `json:"detail"`
	}
	_ = json.Unmarshal(raw, &resp)
	switch resp.Outcome {
	case "updated":
		_ = h.cfg.Repo.UpdateStatus(ctx, app.ID, models.DockerAppStatusRunning, nil)
	case "rolled_back":
		detail := resp.Detail
		_ = h.cfg.Repo.UpdateStatus(ctx, app.ID, models.DockerAppStatusRunning, &detail)
	default:
		_ = h.cfg.Repo.UpdateStatus(ctx, app.ID, models.DockerAppStatusRunning, nil)
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", raw)
}

// requireDockerMarketplaceEnabled rejects every call with 503 when the
// operator hasn't flipped server_settings.docker_marketplace_enabled
// in Server Settings. The /admin/docker-apps/* routes stay MOUNTED
// even with marketplace disabled so the UI can render a clear 503 +
// "enable in Server Settings" hint instead of a generic 404.
func (h *dockerAppHandler) requireDockerMarketplaceEnabled(c *gin.Context) {
	if h.cfg.ServerSettings == nil {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
			"error":  "docker_marketplace_disabled",
			"detail": "server_settings unavailable",
		})
		return
	}
	ss, err := h.cfg.ServerSettings.Get(c.Request.Context())
	if err != nil || ss == nil || !ss.DockerMarketplaceEnabled {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
			"error":  "docker_marketplace_disabled",
			"detail": "enable the Docker App Marketplace in Server Settings",
		})
		return
	}
	c.Next()
}

// firstLineString returns the leading line of a multi-line string.
// Repeated here because cron.go's local helper is unexported and
// adding a cross-file dependency is more cost than the four lines
// of duplication.
func firstLineString(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			return s[:i]
		}
	}
	return s
}


// intToStr is local to avoid pulling strconv into a hot path that
// only uses it for one integer. Phase 6.
func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	digits := make([]byte, 0, 5)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}
