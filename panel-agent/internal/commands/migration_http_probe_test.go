package commands

import (
	"context"
	"encoding/json"
	"testing"
)

func TestMigrationHTTPProbe(t *testing.T) {
	origCurl, origSleep := runProbeCurl, probeSleep
	defer func() { runProbeCurl, probeSleep = origCurl, origSleep }()
	probeSleep = func() {} // no real waiting

	// 200 healthy, 500 crash (flagged), 502 that recovers to 200 on retry,
	// 502 that stays (transient/unhealthy), and an invalid domain (skipped).
	calls := map[string]int{}
	runProbeCurl = func(_ context.Context, d string) int {
		calls[d]++
		switch d {
		case "ok.com":
			return 200
		case "crash.com":
			return 500
		case "warming.com":
			if calls[d] < 2 {
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
