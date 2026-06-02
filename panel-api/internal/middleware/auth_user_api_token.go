// Package middleware — per-user API token bearer-auth (M51).
//
// Authorization header format:
//
//   Authorization: Bearer jat_<43-char base64url>
//
// Server computes sha256 of the FULL token (prefix included), looks
// it up in user_api_tokens by secret_hash, and sets request claims
// to act as the token's owner — identical shape to what
// RequireKratosSession produces from a browser session cookie, so
// every downstream ownership check (claims.UserID == resource.UserID)
// keeps working unmodified.
//
// This middleware composes with RequireKratosSession via
// RequireUserAuth: if a Bearer token is present we use it; otherwise
// we fall through to the Kratos cookie path. The two auth sources
// are mutually exclusive on a single request (a Bearer token wins).
package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/auth"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/ginctx"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/repository"
)

const (
	// userAPITokenPrefix matches what /api/v1/me/api-tokens mints.
	// Prefix-based detection lets pre-receive hooks and secret
	// scanners spot a leaked token in source control.
	userAPITokenPrefix = "jat_"

	// userAPITokenCtxKey stashes the token row on the gin context
	// so downstream handlers (audit log, scope check) can read it
	// without re-querying the DB.
	userAPITokenCtxKey = "jabali_user_api_token"
)

// UserAPIToken returns the verified token from the context, or nil
// when the request was authenticated via Kratos cookie instead.
// Handlers use this for audit logging + scope checks.
func UserAPIToken(c *gin.Context) any {
	v, _ := c.Get(userAPITokenCtxKey)
	return v
}

// extractBearer pulls a `jat_*` token out of the Authorization header.
// Returns empty string when the header is missing or carries some
// other scheme (Kratos cookie path picks it up). Unknown bearer
// prefixes are also ignored — they'll fail downstream auth
// explicitly instead of silently 401'ing here.
func extractBearer(c *gin.Context) string {
	h := c.GetHeader("Authorization")
	if h == "" {
		return ""
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	tok := strings.TrimSpace(parts[1])
	if !strings.HasPrefix(tok, userAPITokenPrefix) {
		return ""
	}
	return tok
}

// hashUserAPIToken is the canonical hash function. Plaintext token
// (prefix included) → sha256 → lowercase hex. The repo's secret_hash
// column stores the same encoding.
func hashUserAPIToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// RequireUserAuth chains Bearer-token auth with Kratos-cookie auth.
// Token wins when both are present. On success the request-scoped
// claims are populated and the next handler runs.
//
// kratos is the fallback. Pass nil to disable the cookie path
// entirely (used by token-only mounts that should NOT accept
// browser auth — e.g. a future API-only subdomain).
func RequireUserAuth(tokens repository.UserAPITokenRepository, users repository.UserRepository, kratos gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if bearer := extractBearer(c); bearer != "" {
			authenticateUserAPIToken(c, tokens, users, bearer)
			return // c.Next was either called by the helper or we aborted
		}
		if kratos != nil {
			kratos(c)
			return
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error":   "missing_credentials",
			"message": "request needs either an Authorization: Bearer jat_… header or a Kratos session cookie",
		})
	}
}

func authenticateUserAPIToken(c *gin.Context, tokens repository.UserAPITokenRepository, users repository.UserRepository, plaintext string) {
	if tokens == nil || users == nil {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
			"error":   "user_api_disabled",
			"message": "user API token authentication is not configured on this panel",
		})
		return
	}

	hash := hashUserAPIToken(plaintext)
	tok, err := tokens.FindByHash(c.Request.Context(), hash)
	if err != nil {
		// Indistinguishable response shape for "no such token" vs
		// "token revoked" — leaking the difference would tell an
		// attacker their guess hit a real but revoked secret.
		if errors.Is(err, repository.ErrNotFound) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "invalid_token",
				"message": "API token is invalid, revoked, or expired",
			})
			return
		}
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
			"error":   "token_lookup_failed",
			"message": "could not validate API token",
		})
		return
	}

	now := time.Now().UTC()
	if !tok.IsActive(now) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error":   "invalid_token",
			"message": "API token is invalid, revoked, or expired",
		})
		return
	}

	// Resolve user row so claims carry is_admin + email + traits the
	// same way the Kratos path would. Without this, ownership checks
	// would still work (they only look at claims.UserID) but admin-
	// gated endpoints would 403 silently.
	panelUser, err := users.FindByID(c.Request.Context(), tok.UserID)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error":   "user_not_found",
			"message": "API token's owner no longer exists",
		})
		return
	}

	claims := &auth.AccessClaims{
		UserID:  panelUser.ID,
		Email:   panelUser.Email,
		IsAdmin: panelUser.IsAdmin,
		// Tag the claims so audit + rate-limiting can tell
		// API-driven from browser-driven traffic.
		Source: auth.SourceUserAPIToken,
	}
	ginctx.SetClaims(c, claims)
	c.Set(userAPITokenCtxKey, tok)

	// Best-effort last_used bump. Don't block the request — async
	// goroutine with a detached context survives client disconnect.
	go func(id, ip string) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tokens.BumpLastUsed(ctx, id, ip, time.Now().UTC())
	}(tok.ID, userAPIClientIP(c))

	c.Next()
}

// clientIP returns the most-trusted client IP available. nginx in
// front sets X-Real-IP; fall back to RemoteAddr (strip the port).
func userAPIClientIP(c *gin.Context) string {
	if v := c.GetHeader("X-Real-IP"); v != "" {
		return v
	}
	if v := c.GetHeader("X-Forwarded-For"); v != "" {
		// First IP in the chain is the original client.
		if i := strings.IndexByte(v, ','); i >= 0 {
			return strings.TrimSpace(v[:i])
		}
		return strings.TrimSpace(v)
	}
	addr := c.Request.RemoteAddr
	if i := strings.LastIndexByte(addr, ':'); i >= 0 {
		return addr[:i]
	}
	return addr
}
