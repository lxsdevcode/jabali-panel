package api

// sso_admin.go — GH #575. Admin SSO maintenance surface.
//
// prune-tokens is a pure DB purge (panel-api owns the DB) so it runs
// in-process — parity with `jabali sso prune-tokens`. Key ROTATION is
// intentionally NOT a one-click endpoint: it re-encrypts every shadow
// password, then needs a filesystem key-file swap + a panel SIGHUP reload,
// and browser-firing an identity-key rotation that restarts the serving
// process is a foot-gun (a bad outcome breaks phpMyAdmin SSO for every
// tenant). The GUI documents the guarded operator procedure instead (the
// CLI-supported branch of #575's acceptance criteria).

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/middleware"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/repository"
)

// SSOAdminHandlerConfig holds the phpMyAdmin SSO token repo for the prune
// endpoint.
type SSOAdminHandlerConfig struct {
	Tokens repository.PhpMyAdminSSOTokenRepository
}

type ssoAdminHandler struct{ cfg SSOAdminHandlerConfig }

// RegisterSSOAdminRoutes mounts /admin/sso/* (admin-only). No-op if Tokens is
// nil (DB unavailable) so the panel still boots.
func RegisterSSOAdminRoutes(g *gin.RouterGroup, cfg SSOAdminHandlerConfig) {
	if cfg.Tokens == nil {
		return
	}
	h := &ssoAdminHandler{cfg: cfg}
	g.POST("/admin/sso/prune-tokens", middleware.RequireAdmin(), h.pruneTokens)
}

// pruneTokens forces an immediate purge of expired phpMyAdmin SSO tokens
// (the reconciler also does this on its nightly sweep). Parity with
// `jabali sso prune-tokens`.
func (h *ssoAdminHandler) pruneTokens(c *gin.Context) {
	n, err := h.cfg.Tokens.PurgeExpired(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"purged": n})
}
