package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestNginxOpsSerialized drives many nginx-mutating handlers concurrently and
// asserts they never overlap in their test→reload critical section (JAB-71).
//
// The mocked nginxTestAndReload bumps a NON-atomic counter on entry and drops it
// on exit, sleeping in between to widen the window. If nginxOpMu failed to
// serialize the handlers, two goroutines would be inside the mock at once —
// which both trips `go test -race` on the unsynchronized counter AND makes
// maxActive exceed 1. With the mutex held across each handler's whole
// write→test→reload, entries are strictly serial: maxActive == 1, no race.
func TestNginxOpsSerialized(t *testing.T) {
	av := t.TempDir()
	en := t.TempDir()
	origAv, origEn, origReload := mailVhostSitesAvailable, mailVhostSitesEnabled, nginxTestAndReload
	mailVhostSitesAvailable = av
	mailVhostSitesEnabled = en

	var active, maxActive int // non-atomic on purpose — nginxOpMu must serialize
	nginxTestAndReload = func(context.Context) error {
		active++
		if active > maxActive {
			maxActive = active
		}
		time.Sleep(time.Millisecond)
		active--
		return nil
	}
	t.Cleanup(func() {
		mailVhostSitesAvailable = origAv
		mailVhostSitesEnabled = origEn
		nginxTestAndReload = origReload
	})

	const workers = 25
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			params, _ := json.Marshal(dockerAppVhostApplyParams{
				DomainName:  fmt.Sprintf("app%d.example.com", i),
				Upstream:    "http://127.0.0.1:10000",
				SSLCertPath: "/etc/ssl/x/fullchain.pem",
				SSLKeyPath:  "/etc/ssl/x/privkey.pem",
				ListenIPv4:  "203.0.113.5",
			})
			if _, err := dockerAppVhostApplyHandler(context.Background(), params); err != nil {
				t.Errorf("apply app%d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	if maxActive != 1 {
		t.Fatalf("nginx ops overlapped: maxActive=%d (want 1) — nginxOpMu did not serialize", maxActive)
	}
}
