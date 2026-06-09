package reconciler

import "context"

// M53 Updates Center — auto-update config reconciler (ADR-0118).
//
// Converges the singleton update_autoupdate_config (DB = truth) onto the host
// by calling the agent's self-gating system.autoupdate_apply every tick. The
// agent only writes files whose bytes changed and only daemon-reloads / flips
// a timer on real drift, so this is cheap to run continuously and self-heals a
// host whose unit files were clobbered (e.g. by a reinstall).
func (r *Reconciler) reconcileAutoupdate(ctx context.Context) {
	if r.updateAutoupdate == nil || r.agent == nil {
		return
	}
	cfg, err := r.updateAutoupdate.EnsureDefault(ctx)
	if err != nil {
		r.log.Debug("autoupdate reconcile: load config failed", "error", err)
		return
	}
	if _, err := r.agent.Call(ctx, "system.autoupdate_apply", map[string]any{
		"apt_enabled":    cfg.AptEnabled,
		"apt_time":       cfg.AptTime,
		"jabali_enabled": cfg.JabaliEnabled,
		"jabali_time":    cfg.JabaliTime,
	}); err != nil {
		r.log.Warn("autoupdate reconcile: agent apply failed", "error", err)
	}
}
