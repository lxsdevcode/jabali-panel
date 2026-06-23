// applications_cache.go — GH #406. PUT /applications/:id/cache {enabled}.
//
// A single owner-scoped switch that drives BOTH layers of caching for a
// WordPress install:
//   - the Redis OBJECT cache (the bundled jabali-wp-cache plugin), gated by a
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
	"log/slog"
	"net/http"
	"path"

	"github.com/gin-gonic/gin"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/ginctx"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/models"
)

type setCacheRequest struct {
	Enabled bool `json:"enabled"`
}

// cacheTenantToken derives the stable per-tenant Redis ACL token.
func cacheTenantToken(secret, osUser string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte("wp-cache-tenant:" + osUser))
	return hex.EncodeToString(m.Sum(nil))
}

// cache toggles the WP object cache + nginx page cache for a WordPress install.
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
	ctx := c.Request.Context()
	installID := c.Param("id")

	// Ownership check (admins may toggle any install).
	var install *models.WordPressInstall
	var err error
	if claims.IsAdmin {
		install, err = h.cfg.ApplicationInstalls.FindByID(ctx, installID)
	} else {
		install, err = h.cfg.ApplicationInstalls.FindByIDAndUserID(ctx, installID, claims.UserID)
	}
	if err != nil {
		if isNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "install_not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}
	if install.AppType != "wordpress" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cache_only_for_wordpress"})
		return
	}

	// Resolve the domain (docroot + name for nginx) and the tenant os user.
	domain, err := h.cfg.Domains.FindByID(ctx, install.DomainID)
	if err != nil {
		slog.ErrorContext(ctx, "cache: domain lookup", "err", err, "domain_id", install.DomainID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}
	var osUser string
	if u, uErr := h.cfg.Users.FindByID(ctx, install.UserID); uErr == nil && u != nil && u.Username != nil {
		osUser = *u.Username
	}
	if osUser == "" {
		slog.ErrorContext(ctx, "cache: user has no linux username", "user_id", install.UserID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}

	// The WP install lives at <docroot>/<subdirectory> (subdir empty => docroot).
	installPath := domain.DocRoot
	if install.Subdirectory != "" {
		installPath = path.Join(domain.DocRoot, install.Subdirectory)
	}

	prefix := "jc:" + osUser + ":" // ACL: ~jc:<osuser>:*

	if req.Enabled {
		// 1. Provision the per-tenant ACL user BEFORE the plugin tries to auth.
		if h.cfg.Redis == nil || h.cfg.CacheTokenSecret == "" {
			// No Redis, or no secret to derive a non-guessable per-tenant token.
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "redis_unavailable"})
			return
		}
		token := cacheTenantToken(h.cfg.CacheTokenSecret, osUser)
		if err := h.provisionTenantACL(ctx, osUser, token); err != nil {
			slog.ErrorContext(ctx, "cache: ACL provision", "err", err, "os_user", osUser)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "acl_provision_failed"})
			return
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": "agent_failed"})
			return
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": "agent_failed"})
			return
		}
	}

	// 3. Couple the nginx page cache: same flag the More-menu toggle writes.
	if err := h.cfg.Domains.UpdateCacheEnabled(ctx, domain.ID, req.Enabled); err != nil {
		slog.ErrorContext(ctx, "cache: domain flag", "err", err, "domain_id", domain.ID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}
	if h.cfg.Reconciler != nil {
		h.cfg.Reconciler.Schedule(domain.ID) // re-render vhost + nginx reload
	}

	// 4. Persist the install-level switch state.
	if err := h.cfg.ApplicationInstalls.UpdateCacheEnabled(ctx, installID, req.Enabled); err != nil {
		slog.ErrorContext(ctx, "cache: install flag", "err", err, "install_id", installID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"cache_enabled": req.Enabled})
}

// provisionTenantACL idempotently creates/updates the per-tenant Redis ACL user
// wp_<osuser> scoped to its own jc:<osuser>:* keyspace, read/write only, no
// dangerous/admin commands. Runs as the jabali_panel client (which holds +acl).
func (h *wordPressHandler) provisionTenantACL(ctx context.Context, osUser, token string) error {
	user := "wp_" + osUser
	// resetkeys/resetchannels make the rule absolute (idempotent re-apply); the
	// keyspace is fenced to ~jc:<osuser>:* — no access to jabali:* / automation:*.
	if err := h.cfg.Redis.Do(ctx, "ACL", "SETUSER", user,
		"on", ">"+token,
		"resetkeys", "~jc:"+osUser+":*",
		"resetchannels",
		"+@read", "+@write", "+@keyspace", "+@connection",
		"-@dangerous",
	).Err(); err != nil {
		return err
	}
	// Persist to the aclfile so it survives a redis restart.
	return h.cfg.Redis.Do(ctx, "ACL", "SAVE").Err()
}
