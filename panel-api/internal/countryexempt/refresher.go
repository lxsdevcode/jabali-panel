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
// sync itself — it exits). Zone snapshots only refetch when stale, so a
// converged tick costs one agent round-trip.
const refreshInterval = time.Minute

// StartRefresher converges the country CIDR allowlist until ctx is done
// (in-process goroutine — repo convention, no external queue). countries
// come from settings on every tick so a PUT or CLI set takes effect
// without a restart. A tick syncs when the selection changed, the last
// sync failed, or the snapshots are stale; a converged steady state
// costs one agent round-trip per minute.
func StartRefresher(ctx context.Context, s *Syncer, countriesFn func(ctx context.Context) []string, log *slog.Logger) {
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
		countries := countriesFn(ctx)
		selection := strings.Join(countries, ",")
		changed := selection != lastSelection
		_, lastErr, running := Status()
		if running {
			continue
		}
		if !changed && lastErr == nil {
			continue
		}
		lastSelection = selection
		// forceRefresh=false: loadCountry refetches expired snapshots
		// itself; force is reserved for the operator's explicit /sync.
		log.Info("countryexempt: refresh tick", "countries", len(countries), "changed", changed, "retry", lastErr != nil)
		KickBackground(log, s, countries, false)
	}
}

// SplitCountries parses the server_settings CSV into a country list.
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
