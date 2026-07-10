package kratosclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// Client.InvalidateIdentity drops cached whoami for one identity; ClearCache
// flushes everything (JAB-3 emergency revoke path).
func TestClient_InvalidateIdentityAndClear(t *testing.T) {
	c := &Client{cache: NewCache(10, time.Minute)}
	c.cache.Set("cookieA", &Identity{ID: "idA"})
	c.cache.Set("cookieB", &Identity{ID: "idB"})

	c.InvalidateIdentity("idA")
	if _, ok := c.cache.Get("cookieA"); ok {
		t.Error("InvalidateIdentity did not drop idA")
	}
	if _, ok := c.cache.Get("cookieB"); !ok {
		t.Error("InvalidateIdentity wrongly dropped idB")
	}

	c.ClearCache()
	if _, ok := c.cache.Get("cookieB"); ok {
		t.Error("ClearCache did not flush idB")
	}

	// Nil-safe: no panic when the client / cache is absent.
	var nilClient *Client
	nilClient.InvalidateIdentity("x")
	nilClient.ClearCache()
	(&Client{}).InvalidateIdentity("x")
	(&Client{}).ClearCache()
}

// Behavioral proof of the AC "cache hit before action, action, then refetch":
// a cached whoami is served without a round-trip, ClearCache forces the next
// Whoami to re-validate upstream, and InvalidateIdentity does the same for the
// matching identity only.
func TestClient_CacheHit_Clear_Refetch(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"sess-1","active":true,"identity":{"id":"idA"}}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, srv.URL)
	ctx := context.Background()

	if _, err := c.Whoami(ctx, "cookieA"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Whoami(ctx, "cookieA"); err != nil { // served from cache
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("upstream hits = %d after 2 Whoami; want 1 (second cached)", got)
	}

	// Emergency clear (revoke path) → next Whoami must re-validate.
	c.ClearCache()
	if _, err := c.Whoami(ctx, "cookieA"); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&hits); got != 2 {
		t.Fatalf("upstream hits = %d after ClearCache + Whoami; want 2 (refetched)", got)
	}

	// Identity-scoped invalidation (suspend/reset path) → refetch too.
	c.InvalidateIdentity("idA")
	if _, err := c.Whoami(ctx, "cookieA"); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&hits); got != 3 {
		t.Fatalf("upstream hits = %d after InvalidateIdentity + Whoami; want 3", got)
	}
}
