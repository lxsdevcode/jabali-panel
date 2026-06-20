// Per-user API token management endpoints (M51).
//
//   GET    /api/v1/me/api-tokens          list the caller's tokens
//   POST   /api/v1/me/api-tokens          mint a new token (plaintext shown once)
//   DELETE /api/v1/me/api-tokens/:id      revoke a token
//
// Authentication for these endpoints themselves is Kratos-cookie
// only — a token cannot mint new tokens or revoke its siblings.
// Enforcing that closes the privilege-escalation path where a leaked
// token would otherwise be able to mint long-lived replacements
// behind the user's back. We check claims.Source for the gate.

package api

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/auth"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/ginctx"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/ids"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/models"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/repository"

	"crypto/sha256"
	"encoding/hex"
)

// MeAPITokensConfig is the dependency set for these handlers. Tokens
// repo is mandatory; without it the routes refuse to mount (registrar
// returns silently in that case so a partial DI graph doesn't crash
// every request).
type MeAPITokensConfig struct {
	Tokens repository.UserAPITokenRepository
}

// RegisterMeAPITokensRoutes mounts the per-user token endpoints under
// `/api/v1/me/api-tokens`. The caller is expected to apply Kratos
// auth before this routegroup; we additionally reject token-auth
// callers in each handler.
func RegisterMeAPITokensRoutes(g *gin.RouterGroup, cfg MeAPITokensConfig) {
	if cfg.Tokens == nil {
		return
	}
	h := &meAPITokensHandler{cfg: cfg}
	grp := g.Group("/me/api-tokens")
	grp.GET("", h.list)
	grp.POST("", h.create)
	grp.DELETE("/:id", h.revoke)
}

type meAPITokensHandler struct {
	cfg MeAPITokensConfig
}

// requireBrowserAuth returns true when the caller used a Kratos
// browser session (i.e. they can mint/revoke their own tokens). A
// token-auth caller gets 403 — see file header for rationale.
func requireBrowserAuth(c *gin.Context) (*auth.AccessClaims, bool) {
	claims := ginctx.Claims(c)
	if claims == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return nil, false
	}
	if claims.Source == auth.SourceUserAPIToken {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error":   "token_cannot_manage_tokens",
			"message": "API tokens cannot mint or revoke other tokens; sign in via the panel UI",
		})
		return nil, false
	}
	return claims, true
}

func (h *meAPITokensHandler) list(c *gin.Context) {
	claims, ok := requireBrowserAuth(c)
	if !ok {
		return
	}
	rows, err := h.cfg.Tokens.ListByUser(c.Request.Context(), claims.UserID)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "list_failed"})
		return
	}
	// rows already JSON-marshal correctly (SecretHash is json:"-").
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

type createTokenRequest struct {
	Name string `json:"name"`
	// ExpiresIn is an optional lifetime in seconds. Omit / 0 for an
	// infinite-lifetime API key the user rotates manually. Capped
	// at one year so a typo doesn't produce a never-rotated token.
	ExpiresInSeconds int64                `json:"expires_in_seconds,omitempty"`
	Scopes           models.UserAPIScopes `json:"scopes,omitempty"`
}

type createTokenResponse struct {
	Token models.UserAPIToken `json:"token"`
	// Secret is the plaintext, shown ONCE in the response. The UI
	// MUST surface a copy-to-clipboard control + warn that the
	// value can't be retrieved later. Subsequent GETs of the token
	// row return SecretLast4 only.
	Secret string `json:"secret"`
}

func (h *meAPITokensHandler) create(c *gin.Context) {
	claims, ok := requireBrowserAuth(c)
	if !ok {
		return
	}
	var req createTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_body"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 100 {
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{
			"error":   "invalid_name",
			"message": "name is required, max 100 chars",
		})
		return
	}

	// Generate the secret. 32 random bytes → base64url ≈ 43 chars,
	// 256 bits of entropy. Prefix gates the token-detection paths
	// in pre-receive hooks + secret scanners.
	const secretBytes = 32
	raw := make([]byte, secretBytes)
	if _, err := rand.Read(raw); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "rng_failed"})
		return
	}
	secret := "jat_" + base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(secret))
	hashHex := hex.EncodeToString(sum[:])
	last4 := secret[len(secret)-4:]

	var expiresAt *time.Time
	if req.ExpiresInSeconds > 0 {
		const maxLifetime = 365 * 24 * 60 * 60 // 1 year cap
		if req.ExpiresInSeconds > maxLifetime {
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{
				"error":   "expiry_too_long",
				"message": "expires_in_seconds is capped at 1 year (31536000)",
			})
			return
		}
		t := time.Now().UTC().Add(time.Duration(req.ExpiresInSeconds) * time.Second)
		expiresAt = &t
	}

	if bad, ok := validateUserScopes(req.Scopes); !ok {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_scope",
			"message": "unknown scope: " + bad,
		})
		return
	}

	tok := &models.UserAPIToken{
		ID:          ids.NewULID(),
		UserID:      claims.UserID,
		Name:        req.Name,
		SecretHash:  hashHex,
		SecretLast4: last4,
		Scopes:      req.Scopes,
		ExpiresAt:   expiresAt,
	}
	if err := h.cfg.Tokens.Create(c.Request.Context(), tok); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "create_failed"})
		return
	}
	c.JSON(http.StatusCreated, createTokenResponse{
		Token:  *tok,
		Secret: secret,
	})
}

func (h *meAPITokensHandler) revoke(c *gin.Context) {
	claims, ok := requireBrowserAuth(c)
	if !ok {
		return
	}
	id := c.Param("id")
	if id == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "missing_id"})
		return
	}
	tok, err := h.cfg.Tokens.FindByID(c.Request.Context(), id)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return
	}
	if tok.UserID != claims.UserID {
		// 404 not 403 — don't leak that the ID exists for someone else.
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return
	}
	if err := h.cfg.Tokens.Revoke(c.Request.Context(), id, time.Now().UTC()); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "revoke_failed"})
		return
	}
	c.Status(http.StatusNoContent)
}
