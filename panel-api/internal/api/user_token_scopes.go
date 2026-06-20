package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/middleware"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/models"
)

// User API token scopes (GH #245, ADR-0144). A token with an EMPTY scope set
// has full per-user access (every existing token — no migration). A token with
// a non-empty set is capability-restricted: it may only reach routes whose
// required scope it Has(), and is denied everywhere else (fail-closed).
//
// Phase 1+2 maps the DNS area + the DDNS shim. Other areas are intentionally
// unmapped: a scoped token is denied there until a later phase maps them — safe
// because scoped tokens are brand new. Full tokens are unaffected throughout.
const (
	ScopeReadDNS  = "read:dns"
	ScopeWriteDNS = "write:dns"
	// ScopeDDNS is the narrowest grant: it permits ONLY the DynDNS shim
	// (/nic/update), not the DNS REST API — ideal for a router credential.
	ScopeDDNS = "ddns"
)

// knownUserScopes is the closed set the token-create API accepts. Rejecting
// unknown scopes stops a typo'd scope from silently fail-closing a token, and
// keeps the vocabulary reviewable as areas are added.
var knownUserScopes = map[string]bool{
	ScopeReadDNS:  true,
	ScopeWriteDNS: true,
	ScopeDDNS:     true,
}

// validateUserScopes returns false (with the offending value) if any requested
// scope is outside the known vocabulary.
func validateUserScopes(scopes models.UserAPIScopes) (string, bool) {
	for _, s := range scopes {
		if !knownUserScopes[s] {
			return s, false
		}
	}
	return "", true
}

// userTokenScopeMap maps "<METHOD> <route-pattern>" (gin's c.FullPath, so
// :params match exactly with no manual parsing) to the scope a restricted token
// must hold. Only routes in this map are reachable by a scoped token.
var userTokenScopeMap = map[string]string{
	"GET /api/v1/domains/:id/dns/zone":           ScopeReadDNS,
	"PATCH /api/v1/domains/:id/dns/zone":         ScopeWriteDNS,
	"GET /api/v1/domains/:id/dns/records":        ScopeReadDNS,
	"POST /api/v1/domains/:id/dns/records":       ScopeWriteDNS,
	"GET /api/v1/domains/:id/dns/system-records": ScopeReadDNS,
	"PATCH /api/v1/dns/records/:recordId":        ScopeWriteDNS,
	"DELETE /api/v1/dns/records/:recordId":       ScopeWriteDNS,
}

// EnforceUserTokenScopes gates the /api/v1 group for restricted API tokens.
// Browser (Kratos cookie) sessions and full tokens (empty scopes) pass through
// untouched; a scoped token is allowed only on a mapped route whose scope it
// holds, and 403'd everywhere else (fail-closed). Mount AFTER the auth
// middleware that stashes the token on the context.
func EnforceUserTokenScopes() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokAny := middleware.UserAPIToken(c)
		if tokAny == nil {
			c.Next() // Kratos cookie auth — already authorized
			return
		}
		tok, ok := tokAny.(*models.UserAPIToken)
		if !ok || len(tok.Scopes) == 0 {
			c.Next() // full token
			return
		}
		want, mapped := userTokenScopeMap[c.Request.Method+" "+c.FullPath()]
		if !mapped {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "scope_not_permitted",
				"message": "this API token is scope-restricted and cannot access this endpoint",
			})
			return
		}
		if !tok.Scopes.Has(want) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "scope_required",
				"message": "API token is missing the required scope: " + want,
			})
			return
		}
		c.Next()
	}
}
