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
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"path"
	"time"

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


// cacheWarmup (GH #615) crawls the install's site (sitemap + homepage) to
// pre-populate the nginx page cache, so operators don't wait for organic
// traffic. Fire-and-forget with a detached context; returns 202 immediately.
func (h *wordPressHandler) cacheWarmup(c *gin.Context) {
	claims := ginctx.Claims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	inst := h.loadOwnedInstall(c, claims)
	if inst == nil {
		return
	}
	if !inst.CacheEnabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cache_not_enabled"})
		return
	}
	dom, err := h.cfg.Domains.FindByID(c.Request.Context(), inst.DomainID)
	if err != nil || dom == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "domain_not_found"})
		return
	}
	if h.cfg.Agent == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent_unavailable"})
		return
	}
	agent, host := h.cfg.Agent, dom.Name
	go func() {
		wctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		_, _ = agent.Call(wctx, "nginx.cache_warmup", map[string]any{"host": host, "max_urls": 100})
	}()
	c.JSON(http.StatusAccepted, gin.H{"ok": true, "warming": host})
}

// cacheProfiles returns the static cache-profile registry for the UI (GH #618).
func (h *wordPressHandler) cacheProfiles(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"profiles": models.CacheProfiles})
}

// cacheStats (GH #617) returns the install's Redis cache stats (hit ratio, key
// count, memory) by asking the agent to run `wp jabali-cache stats` as tenant.
func (h *wordPressHandler) cacheStats(c *gin.Context) {
	claims := ginctx.Claims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	inst := h.loadOwnedInstall(c, claims)
	if inst == nil {
		return
	}
	dom, err := h.cfg.Domains.FindByID(c.Request.Context(), inst.DomainID)
	if err != nil || dom == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "domain_not_found"})
		return
	}
	osUser := ""
	if u, uErr := h.cfg.Users.FindByID(c.Request.Context(), inst.UserID); uErr == nil && u != nil && u.Username != nil {
		osUser = *u.Username
	}
	if osUser == "" || h.cfg.Agent == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "unavailable"})
		return
	}
	installPath := dom.DocRoot
	if inst.Subdirectory != "" {
		installPath = path.Join(dom.DocRoot, inst.Subdirectory)
	}
	res, err := h.cfg.Agent.Call(c.Request.Context(), "wordpress.cache_stats", map[string]any{
		"os_user": osUser, "install_path": installPath,
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "agent_error", "detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"stats": res})
}
