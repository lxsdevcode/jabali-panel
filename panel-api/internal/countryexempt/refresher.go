package countryexempt

import (
	"context"
	"log/slog"
	"strings"
	"time"
)

// refreshInterval is the refresher tick. 60s matches the reconciler's
// convergence model: a CLI-originated change to the country selection is
// picked up within a minute (the CLI process can't run the background
// sync itself — it exits).
const refreshInterval = time.Minute

// verifyInterval is how often a converged selection is re-checked anyway.
// The verify sync is what picks up stale snapshots (>snapshotMaxAge) and —
// more importantly — mmdb updates: the GeoIP DB refreshes regularly on the
// host, and country coverage must track the DB the classifier actually
// reads. A converged verify costs two agent round-trips (mmdb stat +
// allowlist list).
const verifyInterval = 15 * time.Minute

// StartRefresher converges the country CIDR allowlist until ctx is done
// (in-process goroutine — repo convention, no external queue). The
// selection comes from settings on every tick so a PUT or CLI set takes
// effect without a restart. A tick syncs when the selection changed, the
// last sync failed, or the verify interval elapsed.
func StartRefresher(ctx context.Context, s *Syncer, selectionFn func(ctx context.Context) (countries []string, extras []string), log *slog.Logger) {
	if log == nil {
		log = slog.Default()
	}
	t := time.NewTicker(refreshInterval)
	defer t.Stop()
	lastSelection := ""
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		countries, extras := selectionFn(ctx)
		selection := strings.Join(countries, ",") + "|" + strings.Join(extras, ",")
		changed := selection != lastSelection
		lastSuccess, lastErr, running := Status()
		if running {
			continue
		}
		verifyDue := time.Since(lastSuccess) > verifyInterval
		if !changed && lastErr == nil && !verifyDue {
			continue
		}
		lastSelection = selection
		// forceRefresh=false: loadAll refetches expired snapshots and
		// re-derives on mmdb change itself; force is reserved for the
		// operator's explicit /sync.
		log.Info("countryexempt: refresh tick", "countries", len(countries),
			"changed", changed, "retry", lastErr != nil, "verify", verifyDue && !changed)
		KickBackground(log, s, countries, extras, false)
	}
}

// SplitCountries parses the server_settings CSV into a country list. Also
// used for the extra-CIDRs CSV — same shape.
func SplitCountries(csv string) []string {
	if csv == "" {
		return nil
	}
	var out []string
	for _, c := range strings.Split(csv, ",") {
		if c = strings.TrimSpace(c); c != "" {
			out = append(out, c)
		}
	}
	return out
}
