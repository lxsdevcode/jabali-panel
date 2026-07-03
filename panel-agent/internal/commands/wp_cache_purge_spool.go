package commands

// wp_cache_purge_spool.go — GH #611. Auto-purge the nginx FastCGI page cache
// when WordPress content changes. Tenant PHP is unprivileged and cannot delete
// the root-owned nginx cache files, so the jabali-cache plugin drops a small
// purge-request JSON into a shared, sticky spool dir; this root-side agent
// watcher picks it up, validates the requesting tenant actually owns the host,
// and calls the same nginx.cache.purge path (targeted by URL, GH #619).
//
// Same supervised-loop shape as StartLoginAllowlistWatcher (#598): tenant
// writes a request, a trusted root component acts on it — the plugin never
// gets panel credentials or root.

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// wpPurgeSpoolDir is created by install.sh (tmpfs, mode 1777 sticky so any
// tenant can drop a request but only its owner — or root — can remove it).
const wpPurgeSpoolDir = "/run/jabali-wp-purge"

const (
	wpPurgePollInterval = 2 * time.Second
	wpPurgeMaxFileBytes = 8 * 1024 // a purge request is tiny; ignore anything larger.
	wpPurgeMaxPaths     = 64       // cap paths per request.
	wpPurgeMaxPerTick   = 500      // bound work per poll so a flood can't wedge the loop.
)

type wpPurgeRequest struct {
	Host  string   `json:"host"`
	Paths []string `json:"paths"`
}

// StartWpCachePurgeWatcher launches the supervised spool watcher.
func StartWpCachePurgeWatcher(ctx context.Context, log *slog.Logger) {
	if log == nil {
		log = slog.Default()
	}
	// Ensure the spool dir exists with sticky 1777 (tenants create requests,
	// only the owner/root removes them) — so a plain `jabali update` on an
	// existing host enables auto-purge without waiting for a full install.sh run.
	// install.sh's tmpfiles.d entry handles reboot persistence.
	_ = os.MkdirAll(wpPurgeSpoolDir, 0o777)
	_ = os.Chmod(wpPurgeSpoolDir, os.ModeSticky|0o777)
	go func() {
		for {
			if err := ctx.Err(); err != nil {
				return
			}
			runWpPurgeTick(ctx, log)
			select {
			case <-ctx.Done():
				return
			case <-time.After(wpPurgePollInterval):
			}
		}
	}()
}

func runWpPurgeTick(ctx context.Context, log *slog.Logger) {
	entries, err := os.ReadDir(wpPurgeSpoolDir)
	if err != nil {
		return // dir absent until install.sh creates it / first request; not an error.
	}
	n := 0
	for _, e := range entries {
		if n >= wpPurgeMaxPerTick {
			return
		}
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		n++
		path := filepath.Join(wpPurgeSpoolDir, e.Name())
		handleWpPurgeFile(ctx, path, log)
	}
}

// nginxVhostDocroot returns the server docroot from the host's nginx vhost
// (the first uncommented `root` directive). The vhost lives under root-owned
// /etc/nginx and is the panel's authoritative host->docroot mapping (GH #630).
func nginxVhostDocroot(vhostPath string) (string, error) {
	b, err := os.ReadFile(vhostPath)
	if err != nil {
		return "", err
	}
	m := nginxRootRE.FindSubmatch(b)
	if m == nil {
		return "", fmt.Errorf("no root directive in %s", vhostPath)
	}
	return strings.TrimSpace(string(m[1])), nil
}

var nginxRootRE = regexp.MustCompile(`(?m)^\s*root\s+([^;]+);`)

// handleWpPurgeFile validates and processes one spool request, then removes it.
// Every exit path removes the file so a bad/forged request can't accumulate.
func handleWpPurgeFile(ctx context.Context, path string, log *slog.Logger) {
	defer func() { _ = os.Remove(path) }()

	fi, err := os.Stat(path)
	if err != nil || fi.Size() == 0 || fi.Size() > wpPurgeMaxFileBytes {
		return
	}
	// Owner uid → username: the request is only trusted for the tenant that
	// wrote it (sticky dir preserves the writer's uid).
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return
	}
	usr, err := user.LookupId(strconv.FormatUint(uint64(st.Uid), 10))
	if err != nil || usr.Username == "" {
		return
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var req wpPurgeRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return
	}
	if !domainRegex.MatchString(req.Host) {
		return
	}
	// OWNERSHIP (GH #630): validate against PANEL STATE — the root-owned nginx
	// vhost for this host — not a tenant-controllable /home path. The old check
	// stat'd /home/<requester>/domains/<host>/public_html, which a tenant could
	// mkdir to spoof ownership of ANOTHER tenant's host and evict its cache. A
	// tenant cannot forge /etc/nginx/sites-available/<host>.conf, so the vhost's
	// own `root` directive is the authoritative docroot; the requester must own
	// THAT. (req.Host already passed domainRegex above — no path traversal.)
	realDocroot, drerr := nginxVhostDocroot(filepath.Join("/etc/nginx/sites-available", req.Host+".conf"))
	if drerr != nil || realDocroot == "" {
		log.Warn("wp-purge: no nginx vhost for host, ignored", "user", usr.Username, "host", req.Host)
		return
	}
	dst, derr := os.Stat(realDocroot)
	if derr != nil || !dst.IsDir() {
		log.Warn("wp-purge: vhost docroot missing, ignored", "host", req.Host)
		return
	}
	if dstat, ok2 := dst.Sys().(*syscall.Stat_t); !ok2 || dstat.Uid != st.Uid {
		log.Warn("wp-purge: requester does not own the vhost docroot, ignored", "user", usr.Username, "host", req.Host)
		return
	}

	paths := req.Paths
	if len(paths) > wpPurgeMaxPaths {
		paths = paths[:wpPurgeMaxPaths]
	}
	body, _ := json.Marshal(map[string]any{"domain": req.Host, "paths": paths})
	pctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if _, perr := nginxCachePurgeHandler(pctx, body); perr != nil {
		log.Warn("wp-purge: nginx.cache.purge failed", "host", req.Host, "err", perr)
		return
	}
	log.Info("wp-purge: purged nginx cache", "user", usr.Username, "host", req.Host, "paths", len(paths))
}
