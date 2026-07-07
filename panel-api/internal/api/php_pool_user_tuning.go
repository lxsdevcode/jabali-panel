package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/agent"
	ginctx "git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/ginctx"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/repository"
)

// PHPUserTuningHandlerConfig wires the user-facing tiered FPM tuning (GH #339
// phase 2, Steps 5+6). SEPARATE from the admin-only /php-pools routes (PR #34):
// this is user-scoped and every value passes through clampToPackageCap.
type PHPUserTuningHandlerConfig struct {
	Agent               agent.AgentInterface
	Users               repository.UserRepository
	Packages            repository.PackageRepository
	PHPPools            repository.PHPPoolRepository
	PHPPoolIniOverrides repository.PHPPoolIniOverrideRepository
	Modes               repository.PHPPerformanceModeRepository
}

type phpUserTuningHandler struct{ cfg PHPUserTuningHandlerConfig }

// RegisterPHPUserTuningRoutes mounts the user tiers under the (auth-only) group:
//   - GET /me/php-fpm-policy         what the caller is allowed (UI drives off this)
//   - PUT /me/php-performance-mode   L1: pick a preset mode {php_version, mode}
//   - PUT /me/php-pool-tuning        L2: raw pm.* {php_version, ...}, clamped
func RegisterPHPUserTuningRoutes(g *gin.RouterGroup, cfg PHPUserTuningHandlerConfig) {
	h := &phpUserTuningHandler{cfg: cfg}
	g.GET("/me/php-fpm-policy", h.policy)
	g.PUT("/me/php-performance-mode", h.setMode)
	g.PUT("/me/php-pool-tuning", h.setTuning)
}

// ownerPackage resolves the caller's hosting package. Returns nil pkg (not an
// error) when the account has none — treated as "no access".
func (h *phpUserTuningHandler) ownerPackage(c *gin.Context) (*models.User, *models.HostingPackage) {
	claims := ginctx.Claims(c)
	if claims == nil {
		return nil, nil
	}
	user, err := h.cfg.Users.FindByID(c.Request.Context(), claims.UserID)
	if err != nil || user == nil || user.PackageID == nil {
		return user, nil
	}
	pkg, err := h.cfg.Packages.FindByID(c.Request.Context(), *user.PackageID)
	if err != nil {
		return user, nil
	}
	return user, pkg
}

func (h *phpUserTuningHandler) policy(c *gin.Context) {
	_, pkg := h.ownerPackage(c)
	canEdit, advanced := false, false
	var cap, mem uint32
	if pkg != nil {
		canEdit, advanced = pkg.FpmUserCanEdit, pkg.FpmAdvancedMode
		cap, mem = pkg.FpmMaxChildrenCap, pkg.FpmWorkerMemMb
	}
	modes, _ := h.cfg.Modes.GetAll(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{
		"can_edit": canEdit, "advanced": advanced,
		"max_children_cap": cap, "worker_mem_mb": mem, "modes": modes,
	})
}

// userPool loads the caller's pool for a version (own only).
func (h *phpUserTuningHandler) userPool(c *gin.Context, userID, version string) (*models.PHPPool, bool) {
	ctx := c.Request.Context()
	if version != "" {
		if p, err := h.cfg.PHPPools.FindByUserAndVersion(ctx, userID, version); err == nil && p != nil {
			return p, true
		}
	}
	if p, err := h.cfg.PHPPools.FindByUserID(ctx, userID); err == nil && p != nil {
		return p, true
	}
	return nil, false
}

type setModeRequest struct {
	PHPVersion string `json:"php_version"`
	Mode       string `json:"mode"`
}

func (h *phpUserTuningHandler) setMode(c *gin.Context) {
	user, pkg := h.ownerPackage(c)
	if user == nil || pkg == nil || !pkg.FpmUserCanEdit {
		c.JSON(http.StatusForbidden, gin.H{"error": "fpm_editing_not_allowed"})
		return
	}
	var req setModeRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Mode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	preset, err := h.cfg.Modes.Get(c.Request.Context(), req.Mode)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown_mode"})
		return
	}
	pool, ok := h.userPool(c, user.ID, req.PHPVersion)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "pool_not_found"})
		return
	}
	// Preset -> pool, clamped to the package cap.
	mc, st, mn, mx := preset.PmMaxChildren, preset.PmStartServers, preset.PmMinSpareServers, preset.PmMaxSpareServers
	mr, tt := preset.PmMaxRequests, preset.RequestTerminateTimeoutSeconds
	if msg, valid := clampToPackageCap(pkg.FpmMaxChildrenCap, preset.PmMode, &mc, &st, &mn, &mx, &mr, &tt); !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pm_tuning_invalid", "detail": msg})
		return
	}
	h.applyToPool(c, pool, preset.PmMode, mc, st, mn, mx, mr, tt, preset.ProcessIdleTimeoutSeconds, req.Mode)
}

type setTuningRequest struct {
	PHPVersion                     string `json:"php_version"`
	PmMode                         string `json:"pm_mode"`
	PmMaxChildren                  uint32 `json:"pm_max_children"`
	PmStartServers                 uint32 `json:"pm_start_servers"`
	PmMinSpareServers              uint32 `json:"pm_min_spare_servers"`
	PmMaxSpareServers              uint32 `json:"pm_max_spare_servers"`
	PmMaxRequests                  uint32 `json:"pm_max_requests"`
	RequestTerminateTimeoutSeconds uint32 `json:"request_terminate_timeout_seconds"`
	ProcessIdleTimeoutSeconds      uint32 `json:"process_idle_timeout_seconds"`
}

func (h *phpUserTuningHandler) setTuning(c *gin.Context) {
	user, pkg := h.ownerPackage(c)
	if user == nil || pkg == nil || !pkg.FpmAdvancedMode {
		c.JSON(http.StatusForbidden, gin.H{"error": "advanced_mode_not_allowed"})
		return
	}
	var req setTuningRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	pool, ok := h.userPool(c, user.ID, req.PHPVersion)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "pool_not_found"})
		return
	}
	mc, st, mn, mx := req.PmMaxChildren, req.PmStartServers, req.PmMinSpareServers, req.PmMaxSpareServers
	mr, tt := req.PmMaxRequests, req.RequestTerminateTimeoutSeconds
	if mc == 0 {
		mc = 20
	}
	if msg, valid := clampToPackageCap(pkg.FpmMaxChildrenCap, req.PmMode, &mc, &st, &mn, &mx, &mr, &tt); !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pm_tuning_invalid", "detail": msg})
		return
	}
	idle := req.ProcessIdleTimeoutSeconds
	if idle == 0 {
		idle = 60
	}
	h.applyToPool(c, pool, req.PmMode, mc, st, mn, mx, mr, tt, idle, "custom")
}

func (h *phpUserTuningHandler) applyToPool(c *gin.Context, pool *models.PHPPool, pmMode string, mc, st, mn, mx, mr, tt, idle uint32, mode string) {
	pool.PmMode = pmMode
	pool.PmMaxChildren = mc
	pool.PmStartServers = st
	pool.PmMinSpareServers = mn
	pool.PmMaxSpareServers = mx
	pool.PmMaxRequests = mr
	pool.RequestTerminateTimeoutSeconds = tt
	pool.ProcessIdleTimeoutSeconds = idle
	pool.PerformanceMode = mode
	pool.Status = "pending"
	if err := h.cfg.PHPPools.Update(c.Request.Context(), pool); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}
	go reconcilePHPPoolViaAgent(h.cfg.Agent, h.cfg.Users, h.cfg.PHPPoolIniOverrides, h.cfg.PHPPools, pool)
	c.JSON(http.StatusOK, gin.H{"data": pool})
}
