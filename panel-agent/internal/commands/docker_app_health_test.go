package commands

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestProbeLoopbackPorts(t *testing.T) {
	dir := t.TempDir()
	compose := `services:
  app:
    image: x
    ports:
      - "127.0.0.1:10000:80/tcp"
  postgres:
    image: postgres
    # no host port published
  app2:
    ports:
      - "127.0.0.1:10005:3000/tcp"
`
	if err := os.WriteFile(filepath.Join(dir, "compose.yml"), []byte(compose), 0o640); err != nil {
		t.Fatal(err)
	}
	got := probeLoopbackPorts(dir)
	want := map[int]bool{10000: true, 10005: true}
	if len(got) != 2 {
		t.Fatalf("got %v, want 2 ports", got)
	}
	for _, p := range got {
		if !want[p] {
			t.Errorf("unexpected port %d in %v", p, got)
		}
	}
}

func TestProbeLoopbackPorts_NoCompose(t *testing.T) {
	if got := probeLoopbackPorts(t.TempDir()); got != nil {
		t.Errorf("want nil for missing compose.yml, got %v", got)
	}
}

func TestHttpServing(t *testing.T) {
	// A 302 (redirect-to-login) must count as serving — it's <500.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusFound)
	}))
	defer srv.Close()
	port := serverPort(t, srv.URL)

	if !httpServing(context.Background(), []int{port}) {
		t.Error("302 response should count as serving")
	}
	// An unused port → connection refused → not serving.
	if httpServing(context.Background(), []int{1}) {
		t.Error("port 1 should not be serving")
	}
}

func TestHttpServing_5xxNotServing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway) // 502
	}))
	defer srv.Close()
	if httpServing(context.Background(), []int{serverPort(t, srv.URL)}) {
		t.Error("5xx should NOT count as serving")
	}
}

func serverPort(t *testing.T, url string) int {
	t.Helper()
	// url is http://127.0.0.1:PORT
	p := url[strings.LastIndex(url, ":")+1:]
	n, err := strconv.Atoi(p)
	if err != nil {
		t.Fatalf("parse port from %q: %v", url, err)
	}
	return n
}
