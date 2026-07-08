package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/repository"
)

// RequireModule (M353 Phase 1, GH #353) rejects requests to a module's routes
// when that module is disabled in server_settings. The SPA already hides the
// nav + pages, but hiding is not a security boundary — a disabled module's
// endpoints must actually refuse (409) so nothing dispatches to a masked
// service. Flags default ON, so a fresh/upgraded install (or a settings-read
// error) never wrongly blocks a live feature.
//
// A module is named by a predicate over ServerSettings so the caller stays
// decoupled from the flag columns.
type ModuleFlag func(*models.ServerSettings) bool

var (
	ModuleDNS      ModuleFlag = func(s *models.ServerSettings) bool { return s.DNSEnabled }
	ModuleMail     ModuleFlag = func(s *models.ServerSettings) bool { return s.MailEnabled }
	ModuleSecurity ModuleFlag = func(s *models.ServerSettings) bool { return s.SecurityEnabled }
	ModuleQuota    ModuleFlag = func(s *models.ServerSettings) bool { return s.QuotaEnabled }
	ModuleAPI      ModuleFlag = func(s *models.ServerSettings) bool { return s.APIEnabled }
)

// moduleSettingsCache caches the settings row for a short TTL so a module guard
// on a hot route doesn't issue a DB read per request. The flags change only when
// an admin toggles them; a few seconds of staleness is fine and matches the
// SPA's per-session capability cache.
type moduleSettingsCache struct {
	mu      sync.RWMutex
	repo    repository.ServerSettingsRepository
	cached  *models.ServerSettings
	fetched time.Time
	ttl     time.Duration
	nowFunc func() time.Time
}

func (c *moduleSettingsCache) get(ctx *gin.Context) *models.ServerSettings {
	now := c.nowFunc()
	c.mu.RLock()
	if c.cached != nil && now.Sub(c.fetched) < c.ttl {
		s := c.cached
		c.mu.RUnlock()
		return s
	}
	c.mu.RUnlock()

	if c.repo == nil {
		return nil // no repo wired → fail open (module on)
	}
	s, err := c.repo.Get(ctx.Request.Context())
	if err != nil || s == nil {
		// On a read error, do NOT block — return the last-known value, or nil
		// (the guard treats nil as "all modules on" so we fail open, never
		// locking an operator out of a live feature on a transient DB blip).
		c.mu.RLock()
		defer c.mu.RUnlock()
		return c.cached
	}
	c.mu.Lock()
	c.cached, c.fetched = s, now
	c.mu.Unlock()
	return s
}

// NewModuleGuard returns a factory bound to one settings cache, so every
// RequireModule middleware built from it shares a single TTL cache.
func NewModuleGuard(repo repository.ServerSettingsRepository) func(ModuleFlag, string) gin.HandlerFunc {
	cache := &moduleSettingsCache{repo: repo, ttl: 5 * time.Second, nowFunc: time.Now}
	return func(flag ModuleFlag, module string) gin.HandlerFunc {
		return func(c *gin.Context) {
			s := cache.get(c)
			// nil settings (unseeded / read error) → fail open (module on).
			if s != nil && !flag(s) {
				c.AbortWithStatusJSON(http.StatusConflict, gin.H{
					"error":  "module_disabled",
					"module": module,
					"detail": "This module is disabled on this server (Server Settings → Modules).",
				})
				return
			}
			c.Next()
		}
	}
}
