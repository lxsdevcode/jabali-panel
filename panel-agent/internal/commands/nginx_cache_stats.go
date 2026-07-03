package commands

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// nginx_cache_stats.go — GH #617. Tally the per-domain nginx page-cache status
// log (jcache-<domain>.log, one $upstream_cache_status token per line) into
// HIT/MISS/BYPASS/EXPIRED/STALE counts + a hit ratio. Reads a bounded tail of
// the root-owned log; the domain is regex-validated (no traversal).

type nginxCacheStatsParams struct {
	Domain   string `json:"domain"`
	MaxBytes int64  `json:"max_bytes"`
}

func nginxCacheStatsHandler(ctx context.Context, raw json.RawMessage) (any, error) {
	var p nginxCacheStatsParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, csInvalidArg("bad params")
	}
	host := strings.ToLower(strings.TrimSpace(p.Domain))
	if !probeDomainRe.MatchString(host) {
		return nil, csInvalidArg("invalid domain")
	}
	logPath := filepath.Join("/var/log/nginx", "jcache-"+host+".log")
	f, err := os.Open(logPath)
	if err != nil {
		return map[string]any{"available": false, "total": 0, "counts": map[string]int{}, "hit_ratio": 0.0}, nil
	}
	defer f.Close()

	maxBytes := p.MaxBytes
	if maxBytes <= 0 || maxBytes > 8<<20 {
		maxBytes = 2 << 20 // 2 MiB tail
	}
	if fi, sErr := f.Stat(); sErr == nil && fi.Size() > maxBytes {
		_, _ = f.Seek(fi.Size()-maxBytes, io.SeekStart)
	}

	counts := map[string]int{}
	total := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		st := strings.TrimSpace(sc.Text())
		if st == "" || st == "-" {
			continue
		}
		counts[st]++
		total++
	}
	hit := counts["HIT"] + counts["REVALIDATED"]
	cacheable := hit + counts["MISS"] + counts["EXPIRED"] + counts["STALE"] + counts["UPDATING"]
	ratio := 0.0
	if cacheable > 0 {
		ratio = math.Round(float64(hit)/float64(cacheable)*1000) / 10
	}
	return map[string]any{
		"available": true,
		"total":     total,
		"counts":    counts,
		"hit_ratio": ratio,
		"bypass":    counts["BYPASS"],
	}, nil
}

func init() {
	Default.Register("nginx.cache.stats", nginxCacheStatsHandler)
}
