package reconciler

import (
	"context"
	"encoding/json"
	"time"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/models"
)

// M53 Updates Center — run reconciler (ADR-0118).
//
// `jabali update` / apt upgrades run as `systemd-run --no-block` transient
// units, so panel-api never synchronously sees their terminal state. The run
// handler logs an update_history row as `running` at dispatch; this tick flips
// it to success/failed by polling the unit — independent of whether any UI is
// polling, so a row never gets stuck `running` just because the page closed.
//
// Terminal mapping leans on the run handlers dropping `--collect`: a *failed*
// oneshot lingers in systemd "failed" state (readable), while a *successful*
// one is garbage-collected to inactive/unknown. So:
//   active | activating | reloading -> still running (skip)
//   failed (or non-zero ExecMainStatus) -> failed
//   anything else (inactive/unknown/gone) -> success

// statusCmdForUnit maps a run unit to its agent status command. Only the two
// known transient units are pollable; anything else is left alone.
func statusCmdForUnit(unit string) (string, bool) {
	switch unit {
	case "jabali-update-oneshot.service":
		return "system.update_status", true
	case "jabali-apt-oneshot.service":
		return "system.apt_status", true
	}
	return "", false
}

// unitStatusView is the subset of the agent's unit-status reply we need.
type unitStatusView struct {
	Status   string `json:"status"`
	ExitCode *int   `json:"exit_code"`
	LogTail  string `json:"log_tail"`
}

func (r *Reconciler) reconcileUpdateRuns(ctx context.Context) {
	if r.updateRunHistory == nil || r.agent == nil {
		return
	}
	rows, err := r.updateRunHistory.ListRunning(ctx)
	if err != nil {
		r.log.Debug("update-run reconcile: list running failed", "error", err)
		return
	}
	for i := range rows {
		row := rows[i]
		cmd, ok := statusCmdForUnit(row.Unit)
		if !ok {
			continue
		}
		raw, cerr := r.agent.Call(ctx, cmd, map[string]any{
			"since": row.StartedAt.UTC().Format(time.RFC3339),
		})
		if cerr != nil {
			r.log.Debug("update-run reconcile: status call failed", "unit", row.Unit, "error", cerr)
			continue
		}
		var v unitStatusView
		if json.Unmarshal(raw, &v) != nil {
			continue
		}
		switch v.Status {
		case "active", "activating", "reloading":
			continue // still running
		}
		status := models.UpdateStatusSuccess
		summary := "completed"
		if v.Status == "failed" || (v.ExitCode != nil && *v.ExitCode != 0) {
			status = models.UpdateStatusFailed
			summary = "failed"
		}
		excerpt := v.LogTail
		if len(excerpt) > 4000 {
			excerpt = excerpt[len(excerpt)-4000:]
		}
		if err := r.updateRunHistory.MarkFinished(ctx, row.ID, status, summary, excerpt); err != nil {
			r.log.Warn("update-run reconcile: mark finished failed", "id", row.ID, "error", err)
		}
	}
}
