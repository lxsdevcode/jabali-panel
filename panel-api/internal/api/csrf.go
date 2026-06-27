package api

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// EnforceSameOriginForCookieMutations is a global CSRF guard for the
// cookie-authenticated API surface (GH #460).
//
// The panel authenticates browsers with a Kratos session cookie (SameSite=Lax).
// Lax blocks the cookie on most cross-site subrequests, but leaves gaps
// (top-level POST within the browser's Lax-allow window, method confusion), so
// this adds an explicit same-origin check as defense-in-depth before any
// state-changing handler.
//
// Scope:
//   - Safe methods (GET/HEAD/OPTIONS/TRACE) are never blocked.
//   - Requests carrying an `Authorization: Bearer …` token are exempt: a
//     cross-origin page cannot set a custom Authorization header without a CORS
//     preflight it can't satisfy, so token/automation callers are not
//     browser-CSRF-able. Only cookie auth is, and only that path is checked.
//   - For a cookie-authenticated mutating request, the Origin header (or
//     Referer when Origin is absent) host must equal the request Host. A
//     cross-site attacker cannot forge a matching Origin from the victim's
//     browser, so a mismatch — or a request with neither header — is rejected.
//
// Mounted on the /api/v1 group, so it covers every authenticated route
// including the admin subgroups; the separately-mounted HMAC automation group
// is unaffected (signed requests are not CSRF-able).
func EnforceSameOriginForCookieMutations() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
			c.Next()
			return
		}
		if ah := c.GetHeader("Authorization"); strings.HasPrefix(ah, "Bearer ") || strings.HasPrefix(ah, "bearer ") {
			c.Next()
			return
		}
		if !requestIsSameOrigin(c) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "csrf_origin_mismatch"})
			return
		}
		c.Next()
	}
}

// requestIsSameOrigin reports whether the request's Origin (or, when absent,
// Referer) names the same host as the request itself. Host comparison ignores
// the port (hostnameOf) — the security property is that an attacker on another
// domain cannot present the panel's hostname — but is exact hostname equality,
// never substring matching. Both headers absent → false (a genuine browser
// always sends at least one on a cross-/same-origin mutation).
func requestIsSameOrigin(c *gin.Context) bool {
	for _, raw := range []string{c.GetHeader("Origin"), c.GetHeader("Referer")} {
		if raw == "" {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" {
			return false
		}
		return hostnameOf(u.Host) == hostnameOf(c.Request.Host)
	}
	return false
}
