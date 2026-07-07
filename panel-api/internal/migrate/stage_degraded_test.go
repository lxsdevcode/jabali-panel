package migrate

import (
	"testing"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
)

// JAB-38: a degraded job must be resumable — re-running one previously
// dead-ended on `degraded → done` (illegal transition), blocking QA re-runs
// and real recovery after a degraded restore.
func TestDegradedJobIsResumable(t *testing.T) {
	valid := map[string]bool{
		models.MigrationStateDone:      true, // clean re-run completes
		models.MigrationStateDegraded:  true, // still-failing re-run stays degraded
		models.MigrationStateFailed:    true, // hard error
		models.MigrationStateAnalyzing: true, // retry-from-scratch re-entry
		models.MigrationStateRestoring: true,
		models.MigrationStateCancelled: true,
	}
	for to, want := range valid {
		if got := IsValidJobTransition(models.MigrationStateDegraded, to); got != want {
			t.Errorf("IsValidJobTransition(degraded, %s) = %v, want %v", to, got, want)
		}
	}
	// Sanity: the previously-failing transition is now legal.
	if !IsValidJobTransition(models.MigrationStateDegraded, models.MigrationStateDone) {
		t.Fatal("degraded → done must be legal (JAB-38)")
	}
	// Done stays terminal.
	if IsValidJobTransition(models.MigrationStateDone, models.MigrationStateDegraded) {
		t.Error("done → degraded must NOT be a guarded transition (downgrade is a deliberate post-run bypass)")
	}
}
