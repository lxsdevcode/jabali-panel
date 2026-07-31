package middleware

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/sys/unix"
)

// peerConnKey is the context key under which ConnContext stashes the raw
// connection so RequireLocalhost can ask the kernel who is on the other end.
type peerConnKey struct{}

// ConnContext is wired into http.Server.ConnContext so every request can reach
// its own net.Conn. Without it the handler only sees RemoteAddr, which for a
// unix socket is the useless "@" sentinel.
//
//	srv := &http.Server{ ..., ConnContext: middleware.ConnContext }
func ConnContext(ctx context.Context, c net.Conn) context.Context {
	return context.WithValue(ctx, peerConnKey{}, c)
}

// PeerUID reports the UID of the process on the other end of a unix-socket
// request, via SO_PEERCRED. The kernel fills this in at connect time: it cannot
// be set, forged or proxied by the peer, which is exactly why it is the right
// gate for "is this really the local agent?".
//
// ok=false when the request did not arrive over a unix socket, or when
// ConnContext was not wired into the server. Callers must treat !ok as
// "unknown", never as "trusted".
func PeerUID(c *gin.Context) (uint32, bool) {
	conn, _ := c.Request.Context().Value(peerConnKey{}).(net.Conn)
	if conn == nil {
		return 0, false
	}
	uid, err := peerUID(conn)
	if err != nil {
		return 0, false
	}
	return uid, true
}

func peerUID(conn net.Conn) (uint32, error) {
	uc, ok := conn.(*net.UnixConn)
	if !ok {
		return 0, fmt.Errorf("peercred: not a unix connection")
	}
	raw, err := uc.SyscallConn()
	if err != nil {
		return 0, fmt.Errorf("peercred: syscall conn: %w", err)
	}
	var cred *unix.Ucred
	var credErr error
	if err := raw.Control(func(fd uintptr) {
		cred, credErr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	}); err != nil {
		return 0, fmt.Errorf("peercred: control: %w", err)
	}
	if credErr != nil {
		return 0, fmt.Errorf("peercred: getsockopt: %w", credErr)
	}
	return cred.Uid, nil
}

// rootUID is the only peer allowed on the internal routes: jabali-agent runs as
// root. nginx runs as www-data, so a proxied request can never satisfy this.
const rootUID = 0

// requirePeerRoot is the structural half of the internal-route gate. Where the
// header check infers "this came via the proxy", this asks the kernel who
// actually opened the socket — nothing the caller sends can change the answer.
//
// Returns false (and writes the response) when the peer is known and is not
// root. An UNKNOWN peer is left to the caller's other checks rather than
// rejected here, so loopback-TCP deployments keep working.
func requirePeerRoot(c *gin.Context) bool {
	uid, ok := PeerUID(c)
	if !ok {
		return true // not a unix peer (or ConnContext unwired) — other checks apply
	}
	if uid != rootUID {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "localhost_only"})
		return false
	}
	return true
}
