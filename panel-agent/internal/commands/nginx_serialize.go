package commands

import "sync"

// nginxOpMu serializes every nginx-mutating write→`nginx -t`→reload sequence
// across all agent handlers (JAB-71).
//
// The agent runs a goroutine per UDS command and the reconciler converges many
// domains in a single tick, so two handlers could otherwise interleave:
//
//	A: write vhost-A     A: nginx -t (passes)
//	                     B: write vhost-B (broken/partial)
//	A: reload            → nginx loads B's broken config
//
// A test that just passed for A is invalidated by B's write before A reloads,
// so A reloads a config that no longer validates — dropping vhosts until the
// next tick. `nginx -t` already validates the COMPLETE config (all conf.d +
// sites-enabled); the only missing piece is mutual exclusion.
//
// Every handler that mutates nginx config MUST hold this mutex across its ENTIRE
// write→test→reload critical section (not just test+reload) — otherwise a
// concurrent write can still slip in after another handler's test passes. The
// shared `nginxTestAndReload` shim deliberately does NOT take this lock: callers
// already hold it around their write, and a second acquisition here would
// deadlock (sync.Mutex is not reentrant).
var nginxOpMu sync.Mutex
