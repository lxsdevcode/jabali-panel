// impersonation.go — admin act-as grant management (GH #183, ADR-0128).
//
// These endpoints run as the REAL admin: ResolveImpersonation skips the
// /api/v1/admin/impersonation path, so an admin can always create/list/exit a
// grant even while an X-Jabali-Act-As header is set. The effective-user
// override itself lives in middleware.ResolveImpersonation; here we only mint
// and revoke the server-side grant rows.
package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/ginctx"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/ids"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/middleware"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/repository"
)

// impersonationGrantTTL bounds an act-as session server-side. The admin can
// exit sooner (DELETE); FindActive enforces this at read time.
const impersonationGrantTTL = 60 * time.Minute

type ImpersonationHandlerConfig struct {
	Grants repository.ImpersonationGrantRepository
	Users  repository.UserRepository
}

type impersonationHandler struct{ cfg ImpersonationHandlerConfig }

// RegisterImpersonationRoutes mounts the admin-only grant endpoints.
func RegisterImpersonationRoutes(g *gin.RouterGroup, cfg ImpersonationHandlerConfig) {
	if cfg.Grants == nil || cfg.Users == nil {
		panic("api.RegisterImpersonationRoutes: Grants and Users are required")
	}
	h := &impersonationHandler{cfg: cfg}
	grp := g.Group("/admin/impersonation", middleware.RequireAdmin())
	grp.POST("", h.create)
	grp.GET("", h.list)
	grp.DELETE("/:id", h.end)
}

type createImpersonationRequest struct {
	UserID string `json:"user_id"`
}

type impersonationGrantResponse struct {
	ID             string    `json:"id"`
	TargetUserID   string    `json:"target_user_id"`
	TargetEmail    string    `json:"target_email"`
	TargetUsername string    `json:"target_username"`
	ExpiresAt      time.Time `json:"expires_at"`
}

func toGrantResponse(g *models.ImpersonationGrant, targetEmail, targetUsername string) impersonationGrantResponse {
	return impersonationGrantResponse{
		ID:             g.ID,
		TargetUserID:   g.TargetUserID,
		TargetEmail:    targetEmail,
		TargetUsername: targetUsername,
		ExpiresAt:      g.ExpiresAt,
	}
}

// derefStr returns the pointed-to string or "" for a nil pointer.
func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// create mints a grant for (admin → target). Target must exist, be non-admin,
// and not be the admin themselves.
func (h *impersonationHandler) create(c *gin.Context) {
	claims := ginctx.Claims(c) // RequireAdmin guarantees a real admin here
	var req createImpersonationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json", "message": err.Error()})
		return
	}
	if req.UserID == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_argument", "message": "user_id is required"})
		return
	}
	if req.UserID == claims.UserID {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_argument", "message": "cannot impersonate yourself"})
		return
	}
	target, err := h.cfg.Users.FindByID(c.Request.Context(), req.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal", "message": "could not resolve user"})
		return
	}
	if target.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "cannot impersonate an admin"})
		return
	}
	grant := &models.ImpersonationGrant{
		ID:           ids.NewULID(),
		AdminUserID:  claims.UserID,
		TargetUserID: target.ID,
		ExpiresAt:    time.Now().UTC().Add(impersonationGrantTTL),
	}
	if err := h.cfg.Grants.Create(c.Request.Context(), grant); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal", "message": "could not create grant"})
		return
	}
	c.JSON(http.StatusCreated, toGrantResponse(grant, target.Email, derefStr(target.Username)))
}

// list returns the admin's own active grants.
func (h *impersonationHandler) list(c *gin.Context) {
	claims := ginctx.Claims(c)
	grants, err := h.cfg.Grants.ListActiveByAdmin(c.Request.Context(), claims.UserID, time.Now().UTC())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal", "message": "could not list grants"})
		return
	}
	out := make([]impersonationGrantResponse, 0, len(grants))
	for i := range grants {
		email, username := "", ""
		if u, uErr := h.cfg.Users.FindByID(c.Request.Context(), grants[i].TargetUserID); uErr == nil && u != nil {
			email = u.Email
			username = derefStr(u.Username)
		}
		out = append(out, toGrantResponse(&grants[i], email, username))
	}
	c.JSON(http.StatusOK, gin.H{"data": out, "total": len(out)})
}

// end revokes a grant. Only the admin who owns it may end it; ending an
// already-ended/absent grant is a no-op (idempotent exit).
func (h *impersonationHandler) end(c *gin.Context) {
	claims := ginctx.Claims(c)
	id := c.Param("id")
	grant, err := h.cfg.Grants.FindActive(c.Request.Context(), id, time.Now().UTC())
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.Status(http.StatusNoContent) // already gone/expired → idempotent
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal", "message": "could not resolve grant"})
		return
	}
	if grant.AdminUserID != claims.UserID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "not your grant"})
		return
	}
	if err := h.cfg.Grants.End(c.Request.Context(), id, time.Now().UTC()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal", "message": "could not end grant"})
		return
	}
	c.Status(http.StatusNoContent)
}
