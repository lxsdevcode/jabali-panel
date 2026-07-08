package kratosclient

import (
	"context"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"
)

// JAB-6: ExportIdentities must hit the ADMIN socket (NewClient's 2nd arg), not
// the public socket (1st arg). If the args are reversed anywhere, the admin
// listener never sees the request and this fails.
func TestExportIdentities_TargetsAdminSocket(t *testing.T) {
	dir := t.TempDir()
	pubSock := filepath.Join(dir, "public.sock")
	adminSock := filepath.Join(dir, "admin.sock")

	var adminHit, publicHit bool
	serve := func(sock string, hit *bool, body string) func() {
		l, err := net.Listen("unix", sock)
		if err != nil {
			t.Fatalf("listen %s: %v", sock, err)
		}
		mux := http.NewServeMux()
		mux.HandleFunc("/admin/identities", func(w http.ResponseWriter, r *http.Request) {
			*hit = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		})
		srv := &http.Server{Handler: mux, ReadHeaderTimeout: 2 * time.Second}
		go func() { _ = srv.Serve(l) }()
		return func() { _ = srv.Close(); _ = l.Close() }
	}
	defer serve(pubSock, &publicHit, `{"error":"public should not serve admin"}`)()
	defer serve(adminSock, &adminHit, `[]`)()

	// public first, admin second (the correct order).
	c := NewClient("unix:"+pubSock, "unix:"+adminSock)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := c.ExportIdentities(ctx); err != nil {
		t.Fatalf("export failed: %v", err)
	}
	if !adminHit {
		t.Error("admin socket did not receive the export request")
	}
	if publicHit {
		t.Error("export request wrongly hit the public socket")
	}
}
