// applications_cache_settings.go — GH #612/#616/#618. Read/write per-install
// cache tuning (object/page split, TTL, profile, URL exclusions, cookie
// bypass), stored in application_installs.cache_settings (JSON). The Cache
// settings Drawer in the SPA is the consumer; the values flow into the nginx
// page-cache gate at reconcile time (see the vhost template) and into the
// object/page enable split. Owner-scoped exactly like the cache toggle.
package api

import (
	"strings"
	"regexp"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/auth"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/ginctx"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
)

const (
	cacheMaxRules    = 50  // cap URL exclusions / cookie bypass entries
	cacheMaxRuleLen  = 128 // per-entry length cap
	cacheSettingsMax = 8192
)

// getCacheSettings returns the install's parsed settings (defaults when unset).
func (h *wordPressHandler) getCacheSettings(c *gin.Context) {
	claims := ginctx.Claims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	inst := h.loadOwnedInstall(c, claims)
	if inst == nil {
		return
	}
	data, configured := inst.ParseCacheSettings()
	c.JSON(http.StatusOK, gin.H{
		"cache_enabled": inst.CacheEnabled,
		"configured":    configured,
		"settings":      data,
	})
}

// setCacheSettings validates + persists the settings, then schedules a reconcile
// so the nginx gate re-renders with the new rules.
func (h *wordPressHandler) setCacheSettings(c *gin.Context) {
	claims := ginctx.Claims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	inst := h.loadOwnedInstall(c, claims)
	if inst == nil {
		return
	}

	var body models.CacheSettingsData
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	if !validRuleList(body.URLExclusions) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_rules"})
		return
	}

	raw, err := json.Marshal(body)
	if err != nil || len(raw) > cacheSettingsMax {
		c.JSON(http.StatusBadRequest, gin.H{"error": "settings_too_large"})
		return
	}
	if err := h.cfg.ApplicationInstalls.UpdateCacheSettings(c.Request.Context(), inst.ID, raw); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "persist_failed"})
		return
	}

	// GH #612: re-apply so the new object max-TTL / page-TTL take effect now, not
	// only on the next cache toggle. Only when the install is cache-enabled;
	// non-fatal (settings are already persisted, and the reconcile below still
	// re-renders the page gate).
	if inst.CacheEnabled {
		if err := h.setCacheCore(c.Request.Context(), inst.ID, true, claims.IsAdmin, claims.UserID); err != nil {
			slog.WarnContext(c.Request.Context(), "cache: re-apply after settings save", "err", err, "install_id", inst.ID)
		}
	}
	// Re-render the vhost so the new gate/TTL take effect (page cache only).
	if h.cfg.Reconciler != nil {
		if dom, dErr := h.cfg.Domains.FindByID(c.Request.Context(), inst.DomainID); dErr == nil && dom != nil {
			h.cfg.Reconciler.Schedule(dom.ID)
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "settings": body})
}

// loadOwnedInstall resolves the :id install scoped to the caller (admins see
// any), 404s otherwise. Mirrors setCacheCore's ownership rule.
func (h *wordPressHandler) loadOwnedInstall(c *gin.Context, claims *auth.AccessClaims) *models.ApplicationInstall {
	id := c.Param("id")
	var inst *models.ApplicationInstall
	var err error
	if claims.IsAdmin {
		inst, err = h.cfg.ApplicationInstalls.FindByID(c.Request.Context(), id)
	} else {
		inst, err = h.cfg.ApplicationInstalls.FindByIDAndUserID(c.Request.Context(), id, claims.UserID)
	}
	if err != nil || inst == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "install_not_found"})
		return nil
	}
	if inst.AppType != "wordpress" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cache_only_for_wordpress"})
		return nil
	}
	return inst
}

// cacheRuleRE mirrors the agent's bypassPathRE (domain_create.go) so the API
// REJECTS a URL exclusion the agent would silently drop at render time, instead
// of persisting it and confusing the operator (GH #635). Must be an absolute
// path of safe chars.
var cacheRuleRE = regexp.MustCompile(`^/[A-Za-z0-9._/-]{0,127}$`)

func validRuleList(list []string) bool {
	if len(list) > cacheMaxRules {
		return false
	}
	for _, s := range list {
		s = strings.TrimSpace(s)
		if s == "" || len(s) > cacheMaxRuleLen || !cacheRuleRE.MatchString(s) {
			return false
		}
	}
	return true
}
