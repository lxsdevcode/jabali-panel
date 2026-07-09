package main

import (
	"testing"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
)

// JAB-49: degraded is terminal and must be reaped like done/failed/cancelled.
func TestIsTerminal_IncludesDegraded(t *testing.T) {
	for _, st := range []string{
		models.MigrationStateDone, models.MigrationStateDegraded,
		models.MigrationStateFailed, models.MigrationStateCancelled,
	} {
		if !isTerminal(st) {
			t.Errorf("state %q should be terminal", st)
		}
	}
	if isTerminal(models.MigrationStateRestoring) {
		t.Error("restoring must not be terminal")
	}
}
