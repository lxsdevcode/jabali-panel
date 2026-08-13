package countryexempt

import (
	"context"
	"log/slog"
	"strings"
	"time"
)

// refreshInterval is the refresher tick. Each tick re-syncs when the CIDR
// snapshots are older than snapshotMaxAge (zone churn) or the last sync
// failed — a successful converged sync costs one agent round-trip.
const refreshInterval = 24 * time.Hour

// StartRefresher runs the weekly-staleness CIDR refresh until ctx is done
// (in-process goroutine — repo convention, no external queue). countries
// come from settings on every tick so a PUT takes effect without a restart.
func StartRefresher(ctx context.Context, s *Syncer, countriesFn func(ctx context.Context) []string, log *slog.Logger) {
	if log == nil {
		log = slog.Default()
	}
	t := time.NewTicker(refreshInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		countries := countriesFn(ctx)
		if len(countries) == 0 {
			continue // feature off
		}
		lastSuccess, lastErr, running := Status()
		if running {
			continue
		}
		// Refetch when the last successful sync never happened, failed, or
		// the snapshot may be stale; otherwise a cheap no-op convergence
		// check (one agent call) is enough.
		force := lastErr != nil || lastSuccess.IsZero() || time.Since(lastSuccess) > snapshotMaxAge
		log.Info("countryexempt: refresh tick", "countries", len(countries), "force", force)
		KickBackground(log, s, countries, force)
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
