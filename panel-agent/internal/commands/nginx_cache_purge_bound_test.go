package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// JAB-67: a full-domain purge must not scan an unbounded number of shared-cache
// files (cross-tenant DoS). The scanned-file cap stops the walk and reports
// truncation instead of blocking the root agent.
func TestCachePurge_FileCountBounded(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 10; i++ {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("f%d", i)),
			[]byte("KEY: httpsGETother.example.com/\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	origRoot, origMax := fastcgiCacheRoot, cachePurgeMaxFiles
	fastcgiCacheRoot = root
	cachePurgeMaxFiles = 3
	t.Cleanup(func() { fastcgiCacheRoot = origRoot; cachePurgeMaxFiles = origMax })

	run := func(maxFiles int) map[string]any {
		cachePurgeMaxFiles = maxFiles
		params, _ := json.Marshal(nginxCachePurgeParams{Domain: "example.com"})
		resp, err := nginxCachePurgeHandler(context.Background(), params)
		if err != nil {
			t.Fatalf("purge: %v", err)
		}
		return resp.(map[string]any)
	}

	if m := run(3); m["truncated"] != true {
		t.Fatalf("cap=3 over 10 files: expected truncated=true, got %v", m)
	}
	if m := run(1000); m["truncated"] != false {
		t.Fatalf("cap=1000 over 10 files: expected truncated=false, got %v", m)
	}
}
