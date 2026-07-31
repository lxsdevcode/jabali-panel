package middleware

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// The header check in RequireLocalhost infers "this came via the proxy".
// SO_PEERCRED asks the kernel who actually opened the socket, which the peer
// cannot influence at all. This exercises it over a REAL unix socket rather
// than a fabricated gin context, because the whole point is the syscall.
func TestPeerUID_ReportsRealPeerOverUnixSocket(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	sock := filepath.Join(dir, "t.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Skipf("unix socket unavailable: %v", err)
	}
	defer ln.Close()

	type result struct {
		uid uint32
		ok  bool
	}
	got := make(chan result, 1)

	r := gin.New()
	r.GET("/probe", func(c *gin.Context) {
		uid, ok := PeerUID(c)
		got <- result{uid, ok}
		c.Status(http.StatusOK)
	})

	srv := &http.Server{Handler: r, ConnContext: ConnContext}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", sock)
			},
		},
		Timeout: 5 * time.Second,
	}
	resp, err := client.Get("http://unix/probe")
	if err != nil {
		t.Fatalf("request over unix socket: %v", err)
	}
	_ = resp.Body.Close()

	select {
	case res := <-got:
		if !res.ok {
			t.Fatal("PeerUID reported not-ok over a real unix socket — the gate would fall open")
		}
		if want := uint32(os.Getuid()); res.uid != want {
			t.Errorf("PeerUID = %d, want %d (this process opened the connection)", res.uid, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("handler never ran")
	}
}

// Without ConnContext wired into the server there is no conn to inspect, so
// PeerUID must report unknown rather than a bogus uid. requirePeerRoot treats
// unknown as "fall through to the other checks", never as trusted.
func TestPeerUID_UnknownWhenConnContextNotWired(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	c.Request, _ = http.NewRequest(http.MethodGet, "/x", nil)

	if _, ok := PeerUID(c); ok {
		t.Error("PeerUID claimed a known peer with no conn in context")
	}
}

// A non-root peer must be refused. We can only assert the real behaviour for
// the uid this test process actually has; when run as root the gate should
// admit, and when run as an ordinary user it should reject — which is the
// nginx (www-data) case.
func TestRequirePeerRoot_MatchesProcessIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	sock := filepath.Join(dir, "t2.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Skipf("unix socket unavailable: %v", err)
	}
	defer ln.Close()

	allowed := make(chan bool, 1)
	r := gin.New()
	r.GET("/probe", func(c *gin.Context) {
		allowed <- requirePeerRoot(c)
	})
	srv := &http.Server{Handler: r, ConnContext: ConnContext}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	client := &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", sock)
		},
	}, Timeout: 5 * time.Second}
	resp, err := client.Get("http://unix/probe")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	_ = resp.Body.Close()

	select {
	case ok := <-allowed:
		wantAllowed := os.Getuid() == 0
		if ok != wantAllowed {
			t.Errorf("requirePeerRoot = %v for uid %d, want %v", ok, os.Getuid(), wantAllowed)
		}
		if !wantAllowed && ok {
			t.Error("a non-root peer was admitted — nginx (www-data) would get through")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("handler never ran")
	}
	_ = fmt.Sprint()
}
