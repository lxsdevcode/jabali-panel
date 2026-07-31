package middleware

import (
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequireLocalhost rejects every request whose remote address is not on
// the loopback interface OR delivered over the local unix socket. Used
// by panel-api endpoints the agent (or any other local daemon) calls —
// the SPA must never reach them. This is a defence-in-depth check on
// top of the bind-level expectation that panel-api binds to 127.0.0.1
// (legacy) or /run/jabali-panel/api.sock (M25).
//
// Implementation notes:
//   - For TCP peers, split host:port from RemoteAddr and accept only
//     when net.IP.IsLoopback() returns true.
//   - For unix-socket peers, Go's net/http sets RemoteAddr to "@" (the
//     empty abstract address) since unix sockets don't have a remote
//     IP. A unix-socket connection is localhost by definition — the
//     peer process has to be on the same host (or in a mount namespace
//     that bind-mounted the socket, which is equivalent). Accept these.
//   - Empty RemoteAddr is also treated as unix-socket for safety: some
//     adapters drop the "@" sentinel. The net effect is the same since
//     any non-unix peer through an HTTP server will have a RemoteAddr.
func RequireLocalhost() gin.HandlerFunc {
	return func(c *gin.Context) {
		// A request that arrived through nginx is NOT a local caller, even
		// though it reaches us over the same unix socket the agent uses.
		//
		// This was exploitable: panel-api is fronted by nginx over
		// /run/jabali-panel/api.sock (ADR-0050), so EVERY proxied request has
		// RemoteAddr "@" and the unix-socket branch below accepted it
		// unconditionally. The /api/v1/internal/* and malware-ingest routes,
		// whose only gate is this middleware, were therefore reachable
		// unauthenticated from the public internet.
		//
		// nginx always sets these on proxied requests; the agent dialling the
		// socket directly never does. Their presence is positive proof the
		// request came through the proxy, so reject rather than trust the
		// socket. install/nginx also denies these paths outright -- this is the
		// half that does not depend on the vhost staying correct.
		for _, h := range []string{"X-Forwarded-For", "X-Real-IP", "X-Forwarded-Proto", "X-Forwarded-Host"} {
			if c.GetHeader(h) != "" {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "localhost_only"})
				return
			}
		}

		remote := c.Request.RemoteAddr
		// Unix-socket accept: RemoteAddr is "@" or "" (see net/http
		// httputil docs). Accept — the connection is by definition
		// local to this host.
		if remote == "" || remote == "@" {
			c.Next()
			return
		}
		host, _, err := net.SplitHostPort(remote)
		if err != nil {
			// Fallback: some adapters supply a bare host with no port.
			host = remote
		}
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "localhost_only"})
			return
		}
		c.Next()
	}
}
