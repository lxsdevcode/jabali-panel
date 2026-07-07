package commands

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestMigrationHTTPProbe(t *testing.T) {
	origCurl, origSleep := runProbeCurl, probeSleep
	defer func() { runProbeCurl, probeSleep = origCurl, origSleep }()
	probeSleep = func() {} // no real waiting

	// 200 healthy, 500 crash (flagged), 502 that recovers to 200 on retry,
	// 502 that stays (transient/unhealthy), and an invalid domain (skipped).
	// Domains probe in parallel now, so the shared call-counter needs a lock.
	var mu sync.Mutex
	calls := map[string]int{}
	runProbeCurl = func(_ context.Context, d, _ string) int {
		mu.Lock()
		calls[d]++
		n := calls[d]
		mu.Unlock()
		switch d {
		case "ok.com":
			return 200
		case "crash.com":
			return 500
		case "warming.com":
			if n < 2 {
				return 502
			}
			return 200
		case "down.com":
			return 502
		}
		return 0
	}

	params, _ := json.Marshal(migrationHTTPProbeParams{Domains: []string{
		"ok.com", "crash.com", "warming.com", "down.com", "BAD DOMAIN!",
	}})
	out, err := migrationHTTPProbeHandler(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	resp := out.(migrationHTTPProbeResponse)
	got := map[string]httpProbeResult{}
	for _, r := range resp.Results {
		got[r.Domain] = r
	}
	if !got["ok.com"].OK || got["ok.com"].Status != 200 {
		t.Errorf("ok.com: %+v", got["ok.com"])
	}
	if got["crash.com"].OK || got["crash.com"].Status != 500 || got["crash.com"].Note == "" {
		t.Errorf("crash.com should be flagged 500: %+v", got["crash.com"])
	}
	if !got["warming.com"].OK || got["warming.com"].Status != 200 {
		t.Errorf("warming.com should recover on retry: %+v", got["warming.com"])
	}
	if calls["warming.com"] < 2 {
		t.Errorf("warming.com should have retried, calls=%d", calls["warming.com"])
	}
	if got["down.com"].OK {
		t.Errorf("down.com (persistent 502) should be unhealthy: %+v", got["down.com"])
	}
	if got["bad domain!"].Note == "" || got["bad domain!"].OK {
		t.Errorf("invalid domain should be skipped + flagged: %+v", got["bad domain!"])
	}
}

// JAB-31 follow-up: the probe must resolve each domain to the specific IP its
// vhost binds (M24 `listen <IP>:443`), not the box default-route src / loopback,
// or a good site reports a false 0-status failure.
func TestVhostListenIP(t *testing.T) {
	dir := t.TempDir()
	orig := nginxSitesDir
	nginxSitesDir = dir
	defer func() { nginxSitesDir = orig }()

	if err := os.WriteFile(filepath.Join(dir, "specific.com.conf"),
		[]byte("server {\n    listen 192.168.100.86:443 ssl http2;\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "allips.com.conf"),
		[]byte("server {\n    listen 443 ssl;\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := vhostListenIP("specific.com"); got != "192.168.100.86" {
		t.Errorf("specific.com listen IP = %q, want 192.168.100.86", got)
	}
	if got := vhostListenIP("allips.com"); got != "" {
		t.Errorf("allips.com should return \"\" (all addresses), got %q", got)
	}
	if got := vhostListenIP("missing.com"); got != "" {
		t.Errorf("missing vhost should return \"\", got %q", got)
	}
}

func TestProbeResolvesToVhostIP(t *testing.T) {
	dir := t.TempDir()
	origDir := nginxSitesDir
	nginxSitesDir = dir
	origCurl, origSleep := runProbeCurl, probeSleep
	defer func() {
		nginxSitesDir = origDir
		runProbeCurl, probeSleep = origCurl, origSleep
	}()
	probeSleep = func() {}

	_ = os.WriteFile(filepath.Join(dir, "site.com.conf"),
		[]byte("    listen 10.9.8.7:443 ssl;\n"), 0o644)

	var seenIP string
	runProbeCurl = func(_ context.Context, _, ip string) int {
		seenIP = ip
		return 200
	}
	_ = probeOneDomain(context.Background(), "site.com")
	if seenIP != "10.9.8.7" {
		t.Errorf("probe resolved to %q, want the vhost listen IP 10.9.8.7", seenIP)
	}
}
