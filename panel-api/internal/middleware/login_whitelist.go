package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/agent"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/ginctx"
)

// loginWhitelistTTL is how long the CrowdSec allowlist entry lives, and
// loginWhitelistDedup is how often we refresh it. The dedup window is shorter
// than the TTL so an active session keeps the entry alive (re-adding refreshes
// the expiration); an idle session lets it lapse.
const (
	loginWhitelistTTL   = "168h" // 7 days, refreshed on activity
	loginWhitelistDedup = 24 * time.Hour
)

// WhitelistLoginIP adds an authenticated user's client IP to the jabali
// CrowdSec allowlist (time-boxed) the first time a session is seen within the
// dedup window, so a logged-in admin or user is never bounced by CrowdSec from
// the IP they're actively working from. Best-effort and fully out-of-band: the
// add runs in a background goroutine and any missing dependency (Redis, agent)
// downgrades to a no-op — it must never block or fail the request.
//
// Mounted after RequireKratosSession (needs claims). Sibling to
// TrackAdminLogin; fires for ALL authenticated users, not just admins.
func WhitelistLoginIP(rdb *redis.Client, agentCli agent.AgentInterface, log *slog.Logger) gin.HandlerFunc {
	if log == nil {
		log = slog.Default()
	}
	if rdb == nil || agentCli == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		defer c.Next()
		claims := ginctx.Claims(c)
		if claims == nil || claims.UserID == "" {
			return
		}
		cookie, err := c.Cookie("ory_kratos_session")
		if err != nil || cookie == "" {
			return
		}
		ip := clientIP(c)
		if !whitelistableIP(ip) {
			return
		}

		// Dedup per (session, IP) so we don't shell out to cscli on every
		// request — once per dedup window is enough to keep the TTL fresh.
		digest := sha256.Sum256([]byte(cookie + "|" + ip))
		key := "jabali:login-wl-seen:" + hex.EncodeToString(digest[:16])
		setCtx, cancel := context.WithTimeout(c.Request.Context(), 500*time.Millisecond)
		defer cancel()
		ok, err := rdb.SetNX(setCtx, key, "1", loginWhitelistDedup).Result()
		if err != nil {
			log.Debug("login-whitelist: redis setnx failed", "err", err)
			return
		}
		if !ok {
			return
		}

		email := claims.Email
		logger := log
		go func() {
			addCtx, addCancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer addCancel()
			_, err := agentCli.Call(addCtx, "security.crowdsec.allowlists.add", map[string]any{
				"value":      ip,
				"reason":     "auto-whitelist: login " + truncate(email, 120),
				"expiration": loginWhitelistTTL,
			})
			if err != nil {
				// Roll back the dedup guard so the next request retries.
				delCtx, delCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
				defer delCancel()
				_ = rdb.Del(delCtx, key).Err()
				logger.Warn("login-whitelist: allowlist add failed", "ip", ip, "err", err)
			}
		}()
	}
}

// whitelistableIP returns true only for a routable IP worth allowlisting.
// Loopback / unspecified / link-local / private addresses are skipped — they
// are never the target of a CrowdSec public-IP decision, so allowlisting them
// is pointless (and would bloat the list).
func whitelistableIP(s string) bool {
	ip := net.ParseIP(s)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate() {
		return false
	}
	return true
}
