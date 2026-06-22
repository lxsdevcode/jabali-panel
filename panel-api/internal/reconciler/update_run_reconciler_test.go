package reconciler

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/models"
)

// fakeUpdRunRepo is a minimal UpdateHistoryRepository for the run reconciler.
type fakeUpdRunRepo struct {
	running []models.UpdateHistory
	marked  map[string]string
}

func (f *fakeUpdRunRepo) List(context.Context, int) ([]models.UpdateHistory, error) {
	return nil, nil
}
func (f *fakeUpdRunRepo) ListRunning(context.Context) ([]models.UpdateHistory, error) {
	return f.running, nil
}
func (f *fakeUpdRunRepo) Insert(context.Context, *models.UpdateHistory) error { return nil }
func (f *fakeUpdRunRepo) MarkFinished(_ context.Context, id, status, _, _ string) error {
	if f.marked == nil {
		f.marked = map[string]string{}
	}
	f.marked[id] = status
	return nil
}

func newUpdRunReconciler(repo *fakeUpdRunRepo) *Reconciler {
	return &Reconciler{
		updateRunHistory: repo,
		agent:            &fakeAgent{},
		log:              slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}
}

// A running row with no pollable unit can't be reconciled via systemd; once
// clearly stale (panel restart orphaned it) the reconciler must reap it so the
// tasks indicator stops showing a phantom "running" task.
func TestReconcileUpdateRuns_ReapsStaleUnitlessOrphan(t *testing.T) {
	repo := &fakeUpdRunRepo{running: []models.UpdateHistory{{
		ID: "old", Kind: models.UpdateKindJabali, Unit: "",
		Status: models.UpdateStatusRunning, StartedAt: time.Now().Add(-time.Hour),
	}}}
	newUpdRunReconciler(repo).reconcileUpdateRuns(context.Background())
	if repo.marked["old"] != models.UpdateStatusFailed {
		t.Fatalf("stale unit-less orphan must be reaped failed; got %v", repo.marked)
	}
}

// A recent unit-less row is left alone (don't false-fail a just-started run).
func TestReconcileUpdateRuns_KeepsFreshUnitlessRow(t *testing.T) {
	repo := &fakeUpdRunRepo{running: []models.UpdateHistory{{
		ID: "fresh", Kind: models.UpdateKindJabali, Unit: "",
		Status: models.UpdateStatusRunning, StartedAt: time.Now(),
	}}}
	newUpdRunReconciler(repo).reconcileUpdateRuns(context.Background())
	if _, ok := repo.marked["fresh"]; ok {
		t.Fatalf("fresh unit-less row must NOT be reaped yet")
	}
}
