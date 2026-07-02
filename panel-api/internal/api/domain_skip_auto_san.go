// Per-domain "skip auto-added SAN" toggle. Tenants whose mail runs
// elsewhere — or who haven't pointed mail.<domain> + autoconfig.<domain>
// at the box — flip this on so the LE cert covers ONLY <domain> +
// www.<domain> and doesn't fail the whole challenge because of a missing
// subdomain DNS record. Default OFF (panel keeps adding the SANs as
// before).
package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/ginctx"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/repository"
)

type DomainSkipAutoSANConfig struct {
	Domains    repository.DomainRepository
	Reconciler DNSScheduler
}

func RegisterDomainSkipAutoSANRoutes(g *gin.RouterGroup, cfg DomainSkipAutoSANConfig) {
	if cfg.Domains == nil {
		return
	}
	h := &domainSkipAutoSANHandler{cfg: cfg}
	g.GET("/domains/:id/skip-auto-san", h.get)
	g.PUT("/domains/:id/skip-auto-san", h.update)
}

type domainSkipAutoSANHandler struct{ cfg DomainSkipAutoSANConfig }

type skipAutoSANResponse struct {
	DomainID   string `json:"domain_id"`
	DomainName string `json:"domain_name"`
	Enabled    bool   `json:"enabled"`
}

type skipAutoSANUpdateRequest struct {
	Enabled bool `json:"enabled"`
}

func (h *domainSkipAutoSANHandler) get(c *gin.Context) {
	dom, ok := h.resolveDomain(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, skipAutoSANResponse{
		DomainID:   dom.ID,
		DomainName: dom.Name,
		Enabled:    dom.SkipAutoSAN,
	})
}

func (h *domainSkipAutoSANHandler) update(c *gin.Context) {
	dom, ok := h.resolveDomain(c)
	if !ok {
		return
	}
	var req skipAutoSANUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "validation_failed", "detail": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	if err := h.cfg.Domains.UpdateSkipAutoSAN(ctx, dom.ID, req.Enabled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update: " + err.Error()})
		return
	}
	if h.cfg.Reconciler != nil {
		h.cfg.Reconciler.Schedule(dom.ID)
	}
	c.JSON(http.StatusOK, skipAutoSANResponse{
		DomainID:   dom.ID,
		DomainName: dom.Name,
		Enabled:    req.Enabled,
	})
}

func (h *domainSkipAutoSANHandler) resolveDomain(c *gin.Context) (*models.Domain, bool) {
	claims := ginctx.Claims(c)
	if claims == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return nil, false
	}
	dom, err := h.cfg.Domains.FindByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		if isNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "domain_not_found"})
			return nil, false
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return nil, false
	}
	if !claims.IsAdmin && dom.UserID != claims.UserID {
		c.JSON(http.StatusNotFound, gin.H{"error": "domain_not_found"})
		return nil, false
	}
	return dom, true
}
