// applications_cache.go — GH #406. PUT /applications/:id/cache {enabled}.
//
// A single owner-scoped switch that drives BOTH layers of caching for a
// WordPress install:
//   - the Redis OBJECT cache (the bundled jabali-cache plugin), gated by a
//     per-tenant Redis ACL user wp_<osuser> scoped ~jc:<osuser>:* (ADR-0148); and
//   - the nginx FastCGI page cache for the app's domain (ADR-0108), via the same
//     domains.cache_enabled flag the More-menu Caching toggle writes.
//
// The per-tenant ACL token is derived (HMAC(CacheTokenSecret, osuser)) so it is
// STABLE across re-enables and across multiple installs of the same tenant — we
// never have to store or recover it, and re-running ACL SETUSER is idempotent.
package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/ginctx"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
)

type setCacheRequest struct {
	Enabled bool `json:"enabled"`
}

// cacheTenantToken derives the per-tenant Redis ACL token from the global
// secret, the OS user, AND a persisted per-tenant salt (Gitea #415). The salt
// makes the token non-deterministic from (secret, osuser) alone, so a single
// tenant can be rotated (regenerate its salt) without rotating the global
// secret and breaking every other tenant.
func cacheTenantToken(secret, osUser, salt string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte("wp-cache-tenant:" + osUser + ":" + salt))
	return hex.EncodeToString(m.Sum(nil))
}

// cachePathFromSubdir maps a WP install subdirectory to the nginx page-cache
// path prefix (Gitea #420). "" (root install) → "/" (whole domain); "blog" or
// "/blog/" → "/blog".
func cachePathFromSubdir(subdir string) string {
	s := strings.Trim(strings.TrimSpace(subdir), "/")
	if s == "" {
		return "/"
	}
	return "/" + s
}

// revokeTenantRedisACL removes the per-tenant wp_<osUser> Redis ACL user and
// persists the aclfile (GH #408 / ADR-0148 Lifecycle). Idempotent — DELUSER on
// an absent user is a no-op. The caller must ensure the tenant has no remaining
// cache-enabled install, because wp_<osUser> is shared across a tenant's
// installs. Used by both the cache-disable path and the user-delete cascade.
func revokeTenantRedisACL(ctx context.Context, rdb *redis.Client, osUser string) error {
	if rdb == nil || osUser == "" {
		return nil
	}
	if err := rdb.Do(ctx, "ACL", "DELUSER", "wp_"+osUser).Err(); err != nil {
		return err
	}
	return rdb.Do(ctx, "ACL", "SAVE").Err()
}

// cache toggles the WP object cache + nginx page cache for a WordPress install.
// setCache sentinel errors map the cache core's failure modes to the exact
// HTTP statuses/codes the UI depends on (GH #556 extraction — keep in sync
// with the switch in cache() and TestApplicationsCache_Characterization).
var (
	errCacheInstallNotFound  = errors.New("install_not_found")
	errCacheNotWordPress     = errors.New("cache_only_for_wordpress")
	errCacheRedisUnavailable = errors.New("redis_unavailable")
	errCacheSaltFailed       = errors.New("salt_failed")
	errCacheACLProvision     = errors.New("acl_provision_failed")
	errCacheAgentFailed      = errors.New("agent_failed")
)

// cache toggles the WP object cache + nginx page cache for a WordPress install.
// HTTP wrapper: parse request, delegate to setCacheCore, map errors → status.
func (h *wordPressHandler) cache(c *gin.Context) {
	claims := ginctx.Claims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req setCacheRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	err := h.setCacheCore(c.Request.Context(), c.Param("id"), req.Enabled, claims.IsAdmin, claims.UserID)
	switch {
	case err == nil:
		c.JSON(http.StatusOK, gin.H{"cache_enabled": req.Enabled})
	case errors.Is(err, errCacheInstallNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "install_not_found"})
	case errors.Is(err, errCacheNotWordPress):
		c.JSON(http.StatusBadRequest, gin.H{"error": "cache_only_for_wordpress"})
	case errors.Is(err, errCacheRedisUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "redis_unavailable"})
	case errors.Is(err, errCacheSaltFailed):
		c.JSON(http.StatusInternalServerError, gin.H{"error": "salt_failed"})
	case errors.Is(err, errCacheACLProvision):
		c.JSON(http.StatusInternalServerError, gin.H{"error": "acl_provision_failed"})
	case errors.Is(err, errCacheAgentFailed):
		c.JSON(http.StatusInternalServerError, gin.H{"error": "agent_failed"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
	}
}

// SetApplicationCache toggles an install's cache from a non-HTTP caller (CLI,
// GH #556). It runs the SAME core as the web PUT /applications/:id/cache path.
// actorUserID/isAdmin scope ownership exactly as the HTTP handler (an operator
// CLI passes isAdmin=true). Returns the errCache* sentinels so callers can
// message precisely; the periodic reconciler converges the nginx page-cache
// vhost when cfg.Reconciler is nil (the CLI path).
func SetApplicationCache(ctx context.Context, cfg ApplicationHandlerConfig, installID string, enabled, isAdmin bool, actorUserID string) error {
	return (&wordPressHandler{cfg: cfg}).setCacheCore(ctx, installID, enabled, isAdmin, actorUserID)
}

// setCacheCore is the transport-agnostic core of the cache toggle. It returns a
// sentinel error (errCache*) for the caller to map to a status/message; any
// other (wrapped) error is an internal failure (→ 500). Behavior is locked by
// TestApplicationsCache_Characterization.
func (h *wordPressHandler) setCacheCore(ctx context.Context, installID string, enabled, isAdmin bool, actorUserID string) error {
	// Ownership check (admins may toggle any install).
	var install *models.WordPressInstall
	var err error
	if isAdmin {
		install, err = h.cfg.ApplicationInstalls.FindByID(ctx, installID)
	} else {
		install, err = h.cfg.ApplicationInstalls.FindByIDAndUserID(ctx, installID, actorUserID)
	}
	if err != nil {
		if isNotFound(err) {
			return errCacheInstallNotFound
		}
		return fmt.Errorf("install lookup: %w", err)
	}
	if install.AppType != "wordpress" {
		return errCacheNotWordPress
	}

	// Resolve the domain (docroot + name for nginx) and the tenant os user.
	domain, err := h.cfg.Domains.FindByID(ctx, install.DomainID)
	if err != nil {
		slog.ErrorContext(ctx, "cache: domain lookup", "err", err, "domain_id", install.DomainID)
		return fmt.Errorf("domain lookup: %w", err)
	}
	var osUser string
	if u, uErr := h.cfg.Users.FindByID(ctx, install.UserID); uErr == nil && u != nil && u.Username != nil {
		osUser = *u.Username
	}
	if osUser == "" {
		slog.ErrorContext(ctx, "cache: user has no linux username", "user_id", install.UserID)
		return fmt.Errorf("user %s has no linux username", install.UserID)
	}

	// The WP install lives at <docroot>/<subdirectory> (subdir empty => docroot).
	installPath := domain.DocRoot
	if install.Subdirectory != "" {
		installPath = path.Join(domain.DocRoot, install.Subdirectory)
	}

	// Per-site prefix UNDER the per-tenant ACL namespace. The plugin wraps the
	// value as jc:<value>: ; "<osuser>:<installID>" -> jc:<osuser>:<installID>:
	// which the ACL key pattern ~jc:<osuser>:* still matches, while staying unique
	// per install so a tenant's two sites never collide.
	prefix := osUser + ":" + installID

	if enabled {
		// 1. Provision the per-tenant ACL user BEFORE the plugin tries to auth.
		if h.cfg.Redis == nil || h.cfg.CacheTokenSecret == "" {
			return errCacheRedisUnavailable
		}
		salt := ""
		if h.cfg.CacheTokenSalts != nil {
			s2, sErr := h.cfg.CacheTokenSalts.GetOrCreate(ctx, install.UserID)
			if sErr != nil {
				slog.ErrorContext(ctx, "cache: salt get/create", "err", sErr, "user_id", install.UserID)
				return errCacheSaltFailed
			}
			salt = s2
		}
		token := cacheTenantToken(h.cfg.CacheTokenSecret, osUser, salt)
		if err := h.provisionTenantACL(ctx, osUser, token); err != nil {
			slog.ErrorContext(ctx, "cache: ACL provision", "err", err, "os_user", osUser)
			return errCacheACLProvision
		}
		// 2. Agent: stage plugin + write config + activate + enable, as the tenant.
		params := map[string]any{
			"install_path":   installPath,
			"os_user":        osUser,
			"enable":         true,
			"redis_db":       1,
			"prefix":         prefix,
			"redis_password": token,
		}
		if _, err := h.cfg.Agent.Call(ctx, "wordpress.cache_set", params); err != nil {
			slog.ErrorContext(ctx, "cache: agent enable", "err", err, "install_id", installID)
			return errCacheAgentFailed
		}
	} else {
		// Disable the plugin. Leave the ACL user in place — other installs of
		// the same tenant share wp_<osuser>; it's harmless without a config.
		params := map[string]any{
			"install_path": installPath,
			"os_user":      osUser,
			"enable":       false,
		}
		if _, err := h.cfg.Agent.Call(ctx, "wordpress.cache_set", params); err != nil {
			slog.ErrorContext(ctx, "cache: agent disable", "err", err, "install_id", installID)
			return errCacheAgentFailed
		}
	}

	// 3. Couple the nginx page cache (domains.cache_enabled, ADR-0108). Enabling
	// is additive. On DISABLE, a domain can host multiple installs (root +
	// /blog), so only flip the per-domain flag OFF when no sibling install on
	// the same domain still wants it — otherwise we'd kill page cache for a
	// site whose own switch is still ON (GH #409).
	desiredDomainCache := enabled
	if !enabled {
		siblings, sErr := h.cfg.ApplicationInstalls.CountCacheEnabledByDomainID(ctx, domain.ID, installID)
		if sErr != nil {
			// Conservative on error: leave the page cache as-is rather than risk
			// clobbering a sibling. Skip the domain flag write this pass.
			slog.WarnContext(ctx, "cache: count siblings on domain", "err", sErr, "domain_id", domain.ID)
			desiredDomainCache = domain.CacheEnabled
		} else if siblings > 0 {
			desiredDomainCache = true // a sibling still wants the page cache
		}
	}
	// Gitea #420: scope the page cache to this install's path, so enabling cache
	// on a /blog WP install doesn't apply WP-tuned caching to unrelated content
	// elsewhere on the domain. "/" = whole domain (root install).
	pathChanged := false
	if enabled {
		newPath := cachePathFromSubdir(install.Subdirectory)
		pathChanged = newPath != domain.CachePath
		if perr := h.cfg.Domains.UpdateCachePath(ctx, domain.ID, newPath); perr != nil {
			slog.ErrorContext(ctx, "cache: domain cache_path", "err", perr, "domain_id", domain.ID)
		}
	}
	flagChanged := desiredDomainCache != domain.CacheEnabled
	if flagChanged {
		if err := h.cfg.Domains.UpdateCacheEnabled(ctx, domain.ID, desiredDomainCache); err != nil {
			slog.ErrorContext(ctx, "cache: domain flag", "err", err, "domain_id", domain.ID)
			return fmt.Errorf("domain flag: %w", err)
		}
	}
	// GH #600: re-render the vhost when the page-cache PATH changed too, not only
	// when the enabled flag flipped. Enabling a second install (e.g. /blog) on a
	// domain whose cache is already on updates domains.cache_path in the DB but
	// left desiredDomainCache == domain.CacheEnabled, so no reconcile fired and
	// nginx kept serving the old path gate until an unrelated reconcile.
	if h.cfg.Reconciler != nil && (flagChanged || pathChanged) {
		h.cfg.Reconciler.Schedule(domain.ID) // re-render vhost + nginx reload
	}

	// 4. Persist the install-level switch state.
	if err := h.cfg.ApplicationInstalls.UpdateCacheEnabled(ctx, installID, enabled); err != nil {
		slog.ErrorContext(ctx, "cache: install flag", "err", err, "install_id", installID)
		return fmt.Errorf("install flag: %w", err)
	}

	// 5. On disable, reap the per-tenant Redis ACL user once the tenant has no
	// other cache-enabled install (GH #408). wp_<osuser> is shared across a
	// tenant's installs, so it may only be DELUSER'd when this was the last —
	// otherwise siblings lose their cache. Best-effort: a revoke failure is
	// logged, never fails the disable (the toggle already succeeded).
	if !enabled {
		remaining, cErr := h.cfg.ApplicationInstalls.CountCacheEnabledByUserID(ctx, install.UserID, installID)
		if cErr != nil {
			slog.WarnContext(ctx, "cache: count remaining cache-enabled installs", "err", cErr, "user_id", install.UserID)
		} else if remaining == 0 {
			if rErr := revokeTenantRedisACL(ctx, h.cfg.Redis, osUser); rErr != nil {
				slog.WarnContext(ctx, "cache: revoke tenant ACL", "err", rErr, "os_user", osUser)
			}
		}
	}

	// 6. GH #615: pre-warm the freshly-enabled page cache. Fire-and-forget
	// with a background context (the HTTP request returns immediately) and a
	// short delay so the vhost reconcile applies the FastCGI cache directive
	// before we crawl. Best-effort: a warmup failure never affects the toggle.
	if enabled && desiredDomainCache && h.cfg.Agent != nil {
		host := domain.Name
		go func() {
			time.Sleep(20 * time.Second) // let the vhost reconcile land
			wctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			if _, werr := h.cfg.Agent.Call(wctx, "nginx.cache_warmup", map[string]any{"host": host}); werr != nil {
				slog.WarnContext(wctx, "cache: warmup", "err", werr, "host", host)
			}
		}()
	}

	return nil
}

// provisionTenantACL idempotently creates/updates the per-tenant Redis ACL user
// wp_<osuser> scoped to its own jc:<osuser>:* keyspace, read/write only, no
// dangerous/admin commands. Runs as the jabali_panel client (which holds +acl).
// errObjectCacheUnavailable signals that Redis or the cache-token secret isn't
// provisioned (ADR-0148 / #595) — the caller decides whether that's a 503 (the
// toggle) or a silent skip (default-on-install, #597).
var errObjectCacheUnavailable = errors.New("object cache unavailable: redis/secret not provisioned")

// enableObjectCache provisions the per-tenant Redis ACL user, activates the WP
// object-cache plugin for one install (as the tenant, via the agent), and flips
// application_installs.cache_enabled on. Self-contained so both the /cache
// toggle and the auto-enable-on-new-WP-install path (#597) share the SAME
// security-critical steps: the per-tenant ACL fence, the cacheTenantToken
// derivation, and the wordpress.cache_set agent call. It does NOT touch the
// per-domain nginx page cache — that stays an explicit opt-in (ADR-0108).
func (h *wordPressHandler) enableObjectCache(ctx context.Context, install *models.ApplicationInstall) error {
	if h.cfg.Redis == nil || h.cfg.CacheTokenSecret == "" {
		return errObjectCacheUnavailable
	}
	domain, err := h.cfg.Domains.FindByID(ctx, install.DomainID)
	if err != nil {
		return fmt.Errorf("domain lookup: %w", err)
	}
	var osUser string
	if u, uErr := h.cfg.Users.FindByID(ctx, install.UserID); uErr == nil && u != nil && u.Username != nil {
		osUser = *u.Username
	}
	if osUser == "" {
		return fmt.Errorf("user %s has no linux username", install.UserID)
	}
	installPath := domain.DocRoot
	if install.Subdirectory != "" {
		installPath = path.Join(domain.DocRoot, install.Subdirectory)
	}
	prefix := osUser + ":" + install.ID
	salt := ""
	if h.cfg.CacheTokenSalts != nil {
		s2, sErr := h.cfg.CacheTokenSalts.GetOrCreate(ctx, install.UserID)
		if sErr != nil {
			return fmt.Errorf("salt get/create: %w", sErr)
		}
		salt = s2
	}
	token := cacheTenantToken(h.cfg.CacheTokenSecret, osUser, salt)
	if err := h.provisionTenantACL(ctx, osUser, token); err != nil {
		return fmt.Errorf("acl provision: %w", err)
	}
	if _, err := h.cfg.Agent.Call(ctx, "wordpress.cache_set", map[string]any{
		"install_path":   installPath,
		"os_user":        osUser,
		"enable":         true,
		"redis_db":       1,
		"prefix":         prefix,
		"redis_password": token,
	}); err != nil {
		return fmt.Errorf("agent cache_set: %w", err)
	}
	if err := h.cfg.ApplicationInstalls.UpdateCacheEnabled(ctx, install.ID, true); err != nil {
		return fmt.Errorf("mark cache_enabled: %w", err)
	}
	return nil
}

func (h *wordPressHandler) provisionTenantACL(ctx context.Context, osUser, token string) error {
	user := "wp_" + osUser
	// resetkeys/resetchannels make the rule absolute (idempotent re-apply); the
	// keyspace is fenced to ~jc:<osuser>:* — no access to jabali:* / automation:*.
	// Explicit command ALLOWLIST (Gitea #413) instead of the broad +@keyspace
	// category — @keyspace included RANDOMKEY/DBSIZE, which Redis ACLs do NOT
	// pattern-scope, so a tenant could enumerate other tenants' key names /
	// total key count past the ~jc:<osuser>:* fence. This is exactly the set the
	// bundled object cache issues (GET/SET/SETEX/DEL/MGET/INCRBY/DECRBY/SCAN/
	// SELECT/PING/AUTH) plus a little headroom — no RANDOMKEY, DBSIZE, KEYS, or
	// FLUSH. `reset` first makes the rule fully authoritative on re-apply, so an
	// already-provisioned user loses any prior @keyspace grant.
	if err := h.cfg.Redis.Do(ctx, "ACL", "SETUSER", user,
		"reset",
		"on", ">"+token,
		"~jc:"+osUser+":*",
		"+GET", "+SET", "+SETEX", "+PSETEX", "+SETNX", "+DEL", "+UNLINK",
		"+MGET", "+MSET", "+INCR", "+INCRBY", "+DECR", "+DECRBY",
		"+EXPIRE", "+PEXPIRE", "+TTL", "+PTTL", "+PERSIST", "+TYPE", "+EXISTS",
		"+SCAN", "+SELECT", "+PING", "+AUTH", "+HELLO",
	).Err(); err != nil {
		return err
	}
	// Persist to the aclfile so it survives a redis restart.
	return h.cfg.Redis.Do(ctx, "ACL", "SAVE").Err()
}
