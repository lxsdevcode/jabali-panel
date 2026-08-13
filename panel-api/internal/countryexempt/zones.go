// Package countryexempt keeps CrowdSec from ever blocking the operator's
// chosen countries (ADR-0166). It owns the panel-api half of the feature:
//
//   - fetching per-country CIDR sets (ipdeny aggregated zones, IPv4 + IPv6)
//     with a snapshot cache under /var/lib/jabali-panel/country-zones —
//     panel-api does outbound HTTPS because the agent never may (ADR-0050);
//   - diffing the desired CIDR set against the jabali-country-allowlist
//     LAPI AllowList (LAPI is truth — ADR-0061) and pushing deltas to the
//     agent in ≤4000-entry chunks;
//   - a weekly-staleness refresher goroutine (in-process, ctx-tied — repo
//     convention, no external queue).
//
// The agent owns the host side: the s02-enrich GeoIP parser whitelist
// (security.crowdsec.country_exempt.set) and the cscli mechanics
// (security.crowdsec.country_allowlist.sync).
package countryexempt

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultCacheDir is inside panel-api's systemd ReadWritePaths
// (install.sh) so PrivateTmp/ProtectSystem=strict don't block it.
const DefaultCacheDir = "/var/lib/jabali-panel/country-zones"

// snapshotMaxAge is how old a cached zone file may be before the refresher
// refetches. Country ranges churn slowly; weekly is plenty and keeps us
// well inside ipdeny's tolerance for automated fetches.
const snapshotMaxAge = 7 * 24 * time.Hour

// zoneURLTemplates — IPv4 and IPv6 aggregated zones are SEPARATE files on
// ipdeny. Forgetting v6 would leave v6 traffic bannable while v4 passes.
var zoneURLTemplates = []string{
	"https://www.ipdeny.com/ipblocks/data/aggregated/%s-aggregated.zone",
	"https://www.ipdeny.com/ipv6/ipaddresses/aggregated/%s-aggregated.zone",
}

// zoneCache pairs the two snapshot files for a country.
type zoneCache struct {
	dir string
}

func (zc zoneCache) path(code string) string {
	return filepath.Join(zc.dir, strings.ToUpper(code)+".zone")
}

func (zc zoneCache) read(code string) ([]string, time.Time, error) {
	body, err := os.ReadFile(zc.path(code))
	if err != nil {
		return nil, time.Time{}, err
	}
	var st os.FileInfo
	if st, err = os.Stat(zc.path(code)); err != nil {
		return nil, time.Time{}, err
	}
	return parseZone(string(body)), st.ModTime(), nil
}

func (zc zoneCache) write(code string, cidrs []string) error {
	// #nosec G301 — zone snapshots are public data (ipdeny CIDR lists);
	// 0755 matches the neighbouring /var/lib/jabali-panel state dirs.
	if err := os.MkdirAll(zc.dir, 0o755); err != nil {
		return err
	}
	tmp := zc.path(code) + ".tmp"
	// #nosec G306 — same rationale: public, non-secret snapshot data.
	if err := os.WriteFile(tmp, []byte(strings.Join(cidrs, "\n")+"\n"), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, zc.path(code))
}

// parseZone keeps CIDR lines, dropping comments and blanks.
func parseZone(body string) []string {
	var out []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, "/") {
			out = append(out, line)
		}
	}
	return out
}

// fetchCountry downloads both zone files for code (already validated
// ^[A-Z]{2}$ upstream) and returns the merged CIDR list.
func fetchCountry(ctx context.Context, client *http.Client, code string) ([]string, error) {
	lower := strings.ToLower(code)
	var all []string
	for _, tmpl := range zoneURLTemplates {
		url := fmt.Sprintf(tmpl, lower)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetch %s: %w", url, err)
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		_ = resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", url, err)
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("fetch %s: status %d", url, resp.StatusCode)
		}
		all = append(all, parseZone(string(body))...)
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("no CIDRs for country %s", code)
	}
	return all, nil
}

// loadCountry returns the CIDR set for code, preferring a fresh snapshot.
// forceRefresh bypasses the freshness check. On fetch failure a stale
// snapshot is used when present (soft failure — ADR-0166); only a total
// absence of data is an error. stale=true tells the caller the data came
// from a fallback snapshot.
func loadCountry(ctx context.Context, client *http.Client, zc zoneCache, code string, forceRefresh bool) (cidrs []string, stale bool, err error) {
	if !forceRefresh {
		if cached, mtime, rerr := zc.read(code); rerr == nil && time.Since(mtime) < snapshotMaxAge {
			return cached, false, nil
		}
	}
	fresh, ferr := fetchCountry(ctx, client, code)
	if ferr == nil {
		if werr := zc.write(code, fresh); werr != nil {
			// Cache write failure is non-fatal: the sync can proceed with
			// the in-memory set; next tick retries the write.
			return fresh, true, nil
		}
		return fresh, false, nil
	}
	if cached, _, rerr := zc.read(code); rerr == nil {
		return cached, true, nil
	}
	return nil, false, ferr
}
