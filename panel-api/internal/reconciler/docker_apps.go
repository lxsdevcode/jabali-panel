// reconcileDockerApps converges docker_apps rows with the docker
// daemon's view of the world (via the agent's docker_app.* verbs).
//
// State machine (see ADR-0116 + plans/m48-docker-app-marketplace.md):
//
//   pending      -> dispatch docker_app.install; on success move to
//                   `running` (or `unhealthy` if healthchecks failed
//                   within the install verb's wait_healthy budget)
//   installing   -> in-flight install; status-poll to find out
//                   whether it landed in `running` / `failed`
//   running      -> status-poll; if the agent says not-running,
//                   move to `failed` (a crashed-after-install case)
//   stopped      -> no-op; operator owns the next move
//   failed       -> no-op; operator clicks "Retry" to flip back to
//                   pending
//   updating, rolling_back, deleted -> not handled in Phase 3
//                   (Phase 7 ships update/rollback; deletion is
//                   synchronous in the REST handler).
//
// The reconciler only DISPATCHES work; it never blocks waiting for
// the docker daemon. The agent verbs have their own deadlines.
package reconciler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/notifications"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/models"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/dockerapp"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/repository"
)

// reconcileDockerApps is called once per reconciler tick (the same
// cadence as domain reconciliation). The signature matches sibling
// tick functions; the docker-app repo is optional so a deploy without
// M48 wired still boots cleanly.
func (r *Reconciler) reconcileDockerApps(ctx context.Context) {
	if r.dockerApps == nil || r.agent == nil {
		return
	}

	apps, err := r.dockerApps.ListAll(ctx)
	if err != nil {
		r.log.Warn("dockerapp: listAll failed", "err", err)
		return
	}
	for _, app := range apps {
		if app == nil {
			continue
		}
		r.reconcileOneDockerApp(ctx, app)
	}
}

// dockerAppStuckTimeout is the maximum age of a row in a
// transient state (installing / updating) before the watchdog
// forcibly flips it to `failed`. install path uses a 3-minute
// agent timeout; update uses 6. 15 minutes gives ample headroom
// for slow image pulls without leaving zombie rows forever when
// panel-api crashed or the client disconnected mid-flight.
const dockerAppStuckTimeout = 15 * time.Minute

func (r *Reconciler) reconcileOneDockerApp(ctx context.Context, app *models.DockerApp) {
	// Watchdog: any row stuck in a transient state past the timeout
	// is forcibly failed so the operator can delete/retry from the
	// UI. Without this a panel-api restart or a dropped client
	// connection leaves the row at `installing` forever (see
	// updateImage / install handler -- they swallow status-write
	// errors when the request context was cancelled).
	if (app.Status == models.DockerAppStatusInstalling || app.Status == models.DockerAppStatusUpdating) &&
		time.Since(app.UpdatedAt) > dockerAppStuckTimeout {
		msg := fmt.Sprintf("watchdog: stuck in %s for %s", app.Status, time.Since(app.UpdatedAt).Round(time.Second))
		if err := r.dockerApps.UpdateStatus(ctx, app.ID, models.DockerAppStatusFailed, &msg); err != nil {
			r.log.Warn("dockerapp: watchdog status update failed", "id", app.ID, "err", err)
		} else {
			r.log.Warn("dockerapp: watchdog flipped stuck row", "id", app.ID, "slug", app.Slug, "from", app.Status, "age", time.Since(app.UpdatedAt))
		}
		return
	}

	switch app.Status {
	case models.DockerAppStatusPending:
		// Install dispatch is owned by the REST handler when it
		// creates the row (so the operator sees an immediate response).
		// The reconciler only retries when an install crash-loop left
		// the row stuck in pending. Dispatch once per tick at most.
		r.dispatchInstall(ctx, app)
	case models.DockerAppStatusInstalling, models.DockerAppStatusRunning, models.DockerAppStatusStopped:
		// Status-poll; the agent verb is cheap (one docker compose ps).
		// Stopped is included so the row catches up when the operator
		// (or a host-reboot autostart) brought the container back up
		// outside the panel -- otherwise the UI shows stale "stopped"
		// indefinitely.
		r.statusPoll(ctx, app)
		// M48 Phase 7 follow-up: image-digest poll. Cheap (~1 HTTP
		// HEAD via `docker manifest inspect`) but gated by
		// last_check_at so each app gets checked at most once per
		// dockerAppCheckInterval. When the upstream digest moves
		// AND update_mode='auto', dispatch the update flow.
		r.pollImageUpdate(ctx, app)
	default:
		// stopped / failed / deleted / updating / rolling_back are
		// terminal-or-in-flight from this tick's perspective.
	}
}

// dispatchInstall calls docker_app.install with the rendered compose
// the REST handler stashed under the app row. The reconciler is
// idempotent: if the agent already wrote compose.yml, install just
// runs `docker compose up -d` again, which is a no-op when the
// container is already running.
//
// NOTE: in Phase 3 we use a minimal install payload that the agent
// can fulfil from on-disk state alone. The REST handler is the
// primary install driver; this is the recovery path.
func (r *Reconciler) dispatchInstall(ctx context.Context, app *models.DockerApp) {
	// 3-minute deadline for the whole call. docker pull on a fresh
	// host can take a while; the agent has its own internal timeout
	// for the healthcheck wait.
	callCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	// Mark in-flight so a parallel tick doesn't double-dispatch.
	if err := r.dockerApps.UpdateStatus(callCtx, app.ID, models.DockerAppStatusInstalling, nil); err != nil {
		r.log.Warn("dockerapp: status->installing failed", "id", app.ID, "err", err)
		return
	}

	// Bare-minimum recovery payload — slug only. The agent reads
	// compose.yml from disk and brings it up. The full install
	// payload is set by the REST handler on first dispatch.
	raw, err := r.agent.Call(callCtx, "docker_app.install", map[string]any{
		"slug":          app.InstanceSlug,
		"compose_yml":   "RECOVERY",                       // sentinel: agent recovery path reads compose.yml from disk
		"volumes":       []string{},
		"wait_healthy":  false,
		"healthcheck_timeout_seconds": 0,
	})
	if err != nil {
		errMsg := firstLineString(err.Error())
		_ = r.dockerApps.UpdateStatus(ctx, app.ID, models.DockerAppStatusFailed, &errMsg)
		r.log.Warn("dockerapp: install dispatch failed", "id", app.ID, "slug", app.Slug, "err", err)
		return
	}
	r.log.Info("dockerapp: install dispatched", "id", app.ID, "slug", app.Slug, "raw_len", len(raw))
	// Status converges via the next tick's statusPoll.
}

// statusPoll asks the agent for current docker state and writes
// whatever it learns back to the row. Cheap; safe to run on every
// tick.
func (r *Reconciler) statusPoll(ctx context.Context, app *models.DockerApp) {
	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	raw, err := r.agent.Call(callCtx, "docker_app.status", map[string]any{"slug": app.InstanceSlug})
	if err != nil {
		// Don't mark failed — could be a transient agent socket
		// blip. Log + leave the row alone.
		r.log.Debug("dockerapp: status call failed", "id", app.ID, "err", err)
		return
	}
	var resp struct {
		Slug    string `json:"slug"`
		Present bool   `json:"present"`
		Running bool   `json:"running"`
		Health  string `json:"health"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		r.log.Warn("dockerapp: parse status failed", "id", app.ID, "err", err)
		return
	}

	target := app.Status
	switch {
	case !resp.Present:
		// Compose dir is gone — operator probably purged it from
		// shell. We treat as `failed` so the UI doesn't claim it's
		// running.
		target = models.DockerAppStatusFailed
	case resp.Running && (resp.Health == "healthy" || resp.Health == "none"):
		target = models.DockerAppStatusRunning
	case resp.Running:
		// Started but not healthy yet; keep `installing` so the UI
		// shows progress.
		target = models.DockerAppStatusInstalling
	default:
		target = models.DockerAppStatusStopped
	}
	if target == app.Status {
		return
	}
	// Clear last_error when transitioning to a healthy terminal state.
	// Without this, a transient first-install healthcheck timeout leaves
	// the err chip lit forever even after the container becomes ready.
	var errArg *string
	if target == models.DockerAppStatusRunning && app.LastError != nil && *app.LastError != "" {
		empty := ""
		errArg = &empty
	}
	if err := r.dockerApps.UpdateStatus(ctx, app.ID, target, errArg); err != nil {
		r.log.Warn("dockerapp: status update failed", "id", app.ID, "from", app.Status, "to", target, "err", err)
	}
}

// firstLineString is a copy of firstLine but typed for string input,
// avoiding the dependency on reconciler.go's parameter shape.
func firstLineString(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			return s[:i]
		}
	}
	return s
}

// WithDockerApps wires the docker-app repository into the reconciler.
// Phase 1 ships the repo + tables; this hook lights up the tick.
func (r *Reconciler) WithDockerApps(repo repository.DockerAppRepository) *Reconciler {
	r.dockerApps = repo
	return r
}

// WithDockerCatalog wires the loaded M48 catalog so the
// image-update poller can resolve docker_apps.slug to the catalog's
// image_channel. Nil-safe; without it the poller no-ops.
func (r *Reconciler) WithDockerCatalog(cat *dockerapp.Catalog) *Reconciler {
	r.dockerCatalog = cat
	return r
}

// compile-time guard: keep fmt imported even when only used for
// stringification in future Phase 7 work.
var _ = fmt.Sprintf


// dockerAppCheckInterval is the per-app poll cadence for upstream
// image digests. 6h is conservative; busy registries (docker hub
// rate limits) appreciate the lower load and operators don't
// usually need sub-day update awareness.
const dockerAppCheckInterval = 6 * time.Hour

// pollImageUpdate is the per-tick check-for-updates pass. Gated by
// last_check_at so the reconciler can't pound the registry on every
// tick. Sets available_digest when the upstream has moved; when
// update_mode='auto' AND status='running' AND last_error IS NULL,
// dispatches docker_app.update (which the agent runs with snapshot
// + healthcheck + rollback).
func (r *Reconciler) pollImageUpdate(ctx context.Context, app *models.DockerApp) {
	if app == nil || r.dockerCatalog == nil || r.dockerApps == nil {
		return
	}
	if app.LastCheckAt != nil && time.Since(*app.LastCheckAt) < dockerAppCheckInterval {
		return
	}
	entry, ok := r.dockerCatalog.Get(app.Slug)
	if !ok {
		return
	}

	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	raw, err := r.agent.Call(callCtx, "docker_app.check_update", map[string]any{
		"slug":          app.InstanceSlug,
		"image_channel": entry.ImageChannel,
	})
	// Always mark checked even on failure -- we don\'t want a
	// permanently-failing registry to spin the poller hot.
	defer func() {
		_ = r.dockerApps.MarkChecked(context.Background(), app.ID)
	}()
	if err != nil {
		r.log.Debug("dockerapp: check_update failed", "id", app.ID, "err", err)
		return
	}
	var resp struct {
		AvailableDigest string `json:"available_digest"`
		UpdateAvailable bool   `json:"update_available"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		r.log.Warn("dockerapp: parse check_update failed", "id", app.ID, "err", err)
		return
	}
	if !resp.UpdateAvailable {
		// Converge stale state: clear available_digest if it was
		// previously set, so the UI chip drops once upstream caught
		// up (or once an update fully completed and upstream now
		// matches local).
		if app.AvailableDigest != nil && *app.AvailableDigest != "" {
			_ = r.dockerApps.UpdateAvailableDigest(ctx, app.ID, "")
		}
		return
	}
	_ = r.dockerApps.UpdateAvailableDigest(ctx, app.ID, resp.AvailableDigest)
	r.log.Info("dockerapp: update available", "id", app.ID, "slug", app.Slug, "digest", resp.AvailableDigest)

	// Fire docker_app.update_available so the operator's configured
	// channels (email / Slack / ntfy) light up. Title carries the
	// app's display name; body has the digest prefix.
	if r.notificationQueue != nil {
		body := fmt.Sprintf("A new image is available for %s (%s). New digest: %s. Update mode: %s.",
			app.Name, app.Slug, shortDigest(resp.AvailableDigest), app.UpdateMode)
		_, perr := r.notificationQueue.Publish(ctx, notifications.Envelope{
			EventKind: "docker_app.update_available",
			Severity:  "info",
			Title:     fmt.Sprintf("Docker app update available: %s", app.Name),
			Body:      body,
			Deeplink:  "/jabali-admin/docker-apps",
		})
		if perr != nil {
			r.log.Warn("dockerapp: publish update_available failed", "id", app.ID, "err", perr)
		}
	}

	// Auto-update gate. Manual mode just leaves the digest on the
	// row; the UI surfaces it as "update available". Auto dispatches
	// the same agent verb the operator would click.
	if app.UpdateMode == models.DockerAppUpdateModeAuto && app.LastError == nil && app.Status == models.DockerAppStatusRunning {
		r.log.Info("dockerapp: auto-update dispatching", "id", app.ID, "slug", app.Slug)
		updateCtx, ucancel := context.WithTimeout(ctx, 6*time.Minute)
		defer ucancel()
		_, err := r.agent.Call(updateCtx, "docker_app.update", map[string]any{
			"slug":                        app.InstanceSlug,
			"healthcheck_timeout_seconds": 300,
		})
		if err != nil {
			msg := firstLineString(err.Error())
			_ = r.dockerApps.UpdateStatus(ctx, app.ID, models.DockerAppStatusFailed, &msg)
		}
	}
}


// shortDigest trims a `sha256:abcdef...` to a 12-char view for
// notifications and log lines.
func shortDigest(d string) string {
	const prefix = "sha256:"
	if len(d) > len(prefix) {
		d = d[len(prefix):]
	}
	if len(d) > 12 {
		return d[:12]
	}
	return d
}
