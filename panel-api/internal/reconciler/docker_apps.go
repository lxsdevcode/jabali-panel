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

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/models"
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

func (r *Reconciler) reconcileOneDockerApp(ctx context.Context, app *models.DockerApp) {
	switch app.Status {
	case models.DockerAppStatusPending:
		// Install dispatch is owned by the REST handler when it
		// creates the row (so the operator sees an immediate response).
		// The reconciler only retries when an install crash-loop left
		// the row stuck in pending. Dispatch once per tick at most.
		r.dispatchInstall(ctx, app)
	case models.DockerAppStatusInstalling, models.DockerAppStatusRunning:
		// Status-poll; the agent verb is cheap (one docker compose ps).
		r.statusPoll(ctx, app)
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
		"slug":          app.Slug,
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
	raw, err := r.agent.Call(callCtx, "docker_app.status", map[string]any{"slug": app.Slug})
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
	if err := r.dockerApps.UpdateStatus(ctx, app.ID, target, nil); err != nil {
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

// compile-time guard: keep fmt imported even when only used for
// stringification in future Phase 7 work.
var _ = fmt.Sprintf
