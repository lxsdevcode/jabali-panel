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
	"strconv"
	"math"
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
	var stats map[string]any
	if uErr := json.Unmarshal(res, &stats); uErr != nil || stats == nil {
		stats = map[string]any{}
	}
	// GH #617: the tenant ACL can't run INFO, so fill the server-wide hit ratio +
	// memory from the panel's privileged Redis client. ADMIN-ONLY: these numbers
	// are Redis-instance-wide (shared across all tenants), so exposing them to a
	// tenant would leak other tenants' aggregate cache activity (used_memory
	// deltas). Tenants see only their own per-prefix stats (keys/connection).
	if claims.IsAdmin && h.cfg.Redis != nil {
		if info, iErr := h.cfg.Redis.Info(c.Request.Context(), "stats", "memory").Result(); iErr == nil {
			hits := infoInt(info, "keyspace_hits")
			miss := infoInt(info, "keyspace_misses")
			if hits+miss > 0 {
				stats["hit_ratio"] = math.Round(float64(hits)/float64(hits+miss)*1000) / 10
			}
			stats["used_memory"] = infoInt(info, "used_memory")
		}
	}
	// GH #617: nginx page-cache HIT/MISS for THIS domain (the owner's own domain,
	// not cross-tenant), so it's visible to the tenant.
	if h.cfg.Agent != nil {
		if pres, perr := h.cfg.Agent.Call(c.Request.Context(), "nginx.cache.stats", map[string]any{"domain": dom.Name}); perr == nil {
			var pageStats map[string]any
			if json.Unmarshal(pres, &pageStats) == nil && pageStats != nil {
				stats["page_cache"] = pageStats
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"stats": stats})
}

// infoInt extracts an integer field from a Redis INFO text block.
func infoInt(info, key string) int {
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(key) + `:(\d+)`)
	if m := re.FindStringSubmatch(info); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 0
}

// recommendCacheProfile maps a site's active plugins to a cache profile + a
// safe default enable set (GH #620).
func recommendCacheProfile(plugins []string) (profile string, note string) {
	set := map[string]bool{}
	for _, pl := range plugins {
		set[strings.ToLower(strings.TrimSpace(pl))] = true
	}
	switch {
	case set["woocommerce"]:
		return "woocommerce", "WooCommerce detected — cart, checkout, and account pages will always bypass the cache."
	case set["easy-digital-downloads"]:
		return "edd", "Easy Digital Downloads detected — checkout and purchase pages will bypass."
	case set["memberpress"] || set["learndash"] || set["lifterlms"] || set["paid-memberships-pro"] || set["restrict-content-pro"] || set["buddypress"]:
		return "membership_lms", "Membership/LMS plugin detected — member and course pages will bypass."
	default:
		return "", "Brochure/content site — aggressive page caching is safe; only wp-admin/login bypass."
	}
}

// cacheAdvise (GH #620) probes the install and recommends a profile + settings.
func (h *wordPressHandler) cacheAdvise(c *gin.Context) {
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
	res, err := h.cfg.Agent.Call(c.Request.Context(), "wordpress.cache_probe", map[string]any{
		"os_user": osUser, "install_path": installPath, "host": dom.Name,
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "agent_error", "detail": err.Error()})
		return
	}
	var probe struct {
		ActivePlugins   []string `json:"active_plugins"`
		WPVersion       string   `json:"wp_version"`
		TTFBMs          int      `json:"ttfb_ms"`
		PageHitVerified bool     `json:"page_hit_verified"`
		RiskyEndpoints  []string `json:"risky_endpoints"`
	}
	_ = json.Unmarshal(res, &probe)
	profile, note := recommendCacheProfile(probe.ActivePlugins)
	// GH #620: extra findings folded into the note.
	if len(probe.RiskyEndpoints) > 0 {
		note += " Detected dynamic endpoints (" + strings.Join(probe.RiskyEndpoints, ", ") + ") — these must stay bypassed; the chosen profile already excludes them."
	}
	if inst.CacheEnabled && !probe.PageHitVerified {
		note += " Warning: the homepage did not return a page-cache HIT on a repeat request — caching may be bypassed (a cookie/plugin) or not yet warmed."
	}
	// Redis eviction pressure (admin-privileged INFO).
	evicted := 0
	if claims.IsAdmin && h.cfg.Redis != nil {
		if info, iErr := h.cfg.Redis.Info(c.Request.Context(), "stats").Result(); iErr == nil {
			evicted = infoInt(info, "evicted_keys")
			if evicted > 0 {
				note += " Redis is evicting keys under memory pressure (evicted_keys>0) — object-cache entries may be dropped early; consider a smaller max-TTL or more Redis memory."
			}
		}
	}
	// Store the diagnostic on the install so it persists (GH #620).
	settings, _ := inst.ParseCacheSettings()
	settings.Diagnostic = &models.CacheDiagnostic{
		RecommendedProfile: profile, Note: note, TTFBMs: probe.TTFBMs,
		PageHitVerified: probe.PageHitVerified, RiskyEndpoints: probe.RiskyEndpoints,
		RedisEvictedKeys: evicted, WPVersion: probe.WPVersion,
	}
	if raw, mErr := json.Marshal(settings); mErr == nil {
		_ = h.cfg.ApplicationInstalls.UpdateCacheSettings(c.Request.Context(), inst.ID, raw)
	}
	c.JSON(http.StatusOK, gin.H{
		"recommended": gin.H{
			"profile":              profile,
			"object_cache_enabled": true,
			"page_cache_enabled":   true,
			"note":                 note,
		},
		"probe": gin.H{
			"active_plugins": probe.ActivePlugins, "wp_version": probe.WPVersion,
			"ttfb_ms": probe.TTFBMs, "page_hit_verified": probe.PageHitVerified,
			"risky_endpoints": probe.RiskyEndpoints, "redis_evicted_keys": evicted,
		},
	})
}
