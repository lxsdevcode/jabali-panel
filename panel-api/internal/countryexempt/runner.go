package countryexempt

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// syncTimeout bounds one sync run. The US v4+v6 set imports in ~2.5 min
// (measured); an hour is generous headroom for a slow LAPI.
const syncTimeout = time.Hour

// runner serializes every sync path (PUT kick, CLI sync, weekly refresher)
// and coalesces kicks that arrive while a sync is running: only the latest
// (syncer, countries) state is re-applied.
var runner struct {
	mu          sync.Mutex
	running     bool
	pending     bool
	force       bool
	countries   []string
	syncer      *Syncer
	lastSuccess time.Time
	lastErr     error
}

// KickBackground starts a sync in the background (or marks the latest
// state pending when one is already running) and returns immediately.
// The PUT handler uses this so a US-scale import never blocks the request.
func KickBackground(log *slog.Logger, s *Syncer, countries []string, forceRefresh bool) {
	if log == nil {
		log = slog.Default()
	}
	runner.mu.Lock()
	runner.syncer = s
	runner.countries = append([]string(nil), countries...)
	runner.force = runner.force || forceRefresh
	runner.pending = true
	if runner.running {
		runner.mu.Unlock()
		return
	}
	runner.running = true
	runner.mu.Unlock()

	go func() {
		for {
			runner.mu.Lock()
			if !runner.pending {
				runner.running = false
				runner.mu.Unlock()
				return
			}
			runner.pending = false
			s := runner.syncer
			countries := append([]string(nil), runner.countries...)
			force := runner.force
			runner.force = false
			runner.mu.Unlock()

			ctx, cancel := context.WithTimeout(context.Background(), syncTimeout)
			err := s.syncLocked(ctx, countries, force)
			cancel()

			runner.mu.Lock()
			runner.lastErr = err
			if err == nil {
				runner.lastSuccess = time.Now()
			} else {
				log.Warn("countryexempt: sync failed", "err", err)
			}
			runner.mu.Unlock()
		}
	}()
}

// Status reports the runner's last outcome for the refresher's staleness
// check and operator debugging.
func Status() (lastSuccess time.Time, lastErr error, running bool) {
	runner.mu.Lock()
	defer runner.mu.Unlock()
	return runner.lastSuccess, runner.lastErr, runner.running
}
