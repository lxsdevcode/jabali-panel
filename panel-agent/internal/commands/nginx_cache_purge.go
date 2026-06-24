package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"git.linux-hosting.co.il/shukivaknin/jabali2/agentwire"
)

// fastcgiCacheRoot is the shared keyzone storage written by
// install.sh's install_nginx_fastcgi_cache (ADR-0108). Kept in lockstep
// with /etc/nginx/conf.d/jabali-fastcgi-cache.conf's fastcgi_cache_path.
const fastcgiCacheRoot = "/var/cache/nginx/jabali"

type nginxCachePurgeParams struct {
	Domain string `json:"domain"`
}

// nginxCachePurgeHandler clears one domain's entries from the shared
// FastCGI keyzone (ADR-0108 v1: host-key grep-unlink — the cache_key is
// "$scheme$request_method$host$request_uri", so every cached file's
// stored `KEY: …` line contains the host). No nginx reload needed:
// nginx re-MISSes deleted entries transparently. Full per-domain cache
// partitioning is a documented follow-up.
func nginxCachePurgeHandler(ctx context.Context, params json.RawMessage) (any, error) {
	var p nginxCachePurgeParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &agentwire.AgentError{
			Code:    agentwire.CodeInvalidArgument,
			Message: fmt.Sprintf("failed to parse params: %v", err),
		}
	}
	// Reuse the strict domain validator (same one domain.create uses).
	// The domain is embedded in the KEY-match below; sanitising it here
	// makes the byte match safe and rejects traversal/glob input.
	if !domainRegex.MatchString(p.Domain) {
		return nil, csInvalidArg(fmt.Sprintf("invalid domain %q", p.Domain))
	}

	root := fastcgiCacheRoot
	if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
		// Cache dir absent ⇒ nothing cached for anyone yet. Not an
		// error: a purge with no cache is a successful no-op.
		return map[string]any{"ok": true, "purged": 0, "domain": p.Domain}, nil
	}

	// nginx stores a `KEY: <scheme><method><host><uri>` line in the cache file
	// header (cache_key = $scheme$request_method$host$request_uri, concatenated
	// with no delimiter). Match the HOST FIELD exactly — a raw substring match
	// would evict another tenant whose host merely CONTAINS this domain
	// ("a.com" ⊂ "sub.a.com") or whose URL path contains the string, a
	// cross-tenant cache-eviction lever despite the owner-scoped REST (Gitea #418).
	needle := []byte("KEY: ")
	host := p.Domain
	purged := 0
	walkErr := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil //nolint:nilerr // skip unreadable entries, keep walking
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		// Cache files are small headers + body; read a bounded prefix
		// to find the KEY line without slurping large bodies.
		f, oerr := os.Open(path)
		if oerr != nil {
			return nil
		}
		buf := make([]byte, 4096)
		n, _ := f.Read(buf)
		_ = f.Close()
		head := buf[:n]
		if i := bytes.Index(head, needle); i >= 0 {
			line := head[i:]
			if j := bytes.IndexByte(line, '\n'); j >= 0 {
				line = line[:j]
			}
			if keyLineHost(line) == host {
				if rmErr := os.Remove(path); rmErr == nil {
					purged++
				}
			}
		}
		return nil
	})
	if walkErr != nil && walkErr != context.Canceled && walkErr != context.DeadlineExceeded {
		return nil, csInternal("walk cache dir", walkErr)
	}
	return map[string]any{"ok": true, "purged": purged, "domain": p.Domain}, nil
}

// keyLineHost extracts the $host field from an nginx cache KEY line of the form
// "KEY: <scheme><method><host><uri>". The fields are concatenated with no
// separator, so we peel the known scheme + method prefixes and take everything
// up to the first "/" (the request_uri always begins with "/"). Returns "" if
// the line doesn't parse. Exact host comparison (not substring) is the #418 fix.
func keyLineHost(line []byte) string {
	v := strings.TrimSpace(strings.TrimPrefix(string(line), "KEY:"))
	for _, sch := range []string{"https", "http"} { // https first (longer)
		if strings.HasPrefix(v, sch) {
			v = v[len(sch):]
			break
		}
	}
	for _, m := range []string{"GET", "HEAD", "POST", "PUT", "DELETE", "OPTIONS", "PATCH", "PURGE", "TRACE", "CONNECT"} {
		if strings.HasPrefix(v, m) {
			v = v[len(m):]
			break
		}
	}
	if i := strings.IndexByte(v, '/'); i >= 0 {
		return v[:i]
	}
	return v
}

func init() {
	Default.Register("nginx.cache.purge", nginxCachePurgeHandler)
}
