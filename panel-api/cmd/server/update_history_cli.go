package main

import (
	"context"
	"os"
	"time"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/repository"
)

// beginUpdateHistory records a self-update run in update_history so that
// scheduled (jabali-autoupdate.timer) and manual-CLI `jabali update` runs show
// up in the panel's Recent Update History — previously only panel-triggered
// "Update now" runs were logged, so an automatic self-update left no trace
// (GH #300 follow-up). It returns a finish(err) closure to call when the
// update completes.
//
// Entirely best-effort: any config/DB problem returns no-op closures and never
// affects the update itself (this is the recovery path — it must stay robust).
//
// The panel "Update now" path already inserts a running row via the API before
// launching `jabali update -f`; its transient unit sets
// JABALI_UPDATE_FROM_PANEL=1, so we skip here to avoid a duplicate row.
func beginUpdateHistory() func(error) {
	noop := func(error) {}
	if os.Getenv("JABALI_UPDATE_FROM_PANEL") != "" {
		return noop
	}
	if err := initConfig(); err != nil {
		return noop
	}
	if err := initDB(); err != nil {
		return noop
	}
	repo := repository.NewUpdateHistoryRepository(sharedDB)
	row := &models.UpdateHistory{
		Kind:      models.UpdateKindJabali,
		Action:    models.UpdateActionRun,
		Status:    models.UpdateStatusRunning,
		Unit:      "jabali-autoupdate",
		Summary:   "self-update via jabali update",
		StartedAt: time.Now(),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := repo.Insert(ctx, row); err != nil {
		return noop
	}
	id := row.ID
	return func(runErr error) {
		status := models.UpdateStatusSuccess
		summary := "self-update completed"
		if runErr != nil {
			status = models.UpdateStatusFailed
			summary = "self-update failed: " + runErr.Error()
		}
		if len(summary) > 255 {
			summary = summary[:255]
		}
		fctx, fcancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer fcancel()
		_ = repo.MarkFinished(fctx, id, status, summary, "")
	}
}
