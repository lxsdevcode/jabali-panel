package main

import (
	"testing"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
)

// JAB-31: a restore that runs to completion but leaves a core area empty must be
// treated as degraded, not done. Lock the exact thresholds + the downgrade gate.
func TestMigrationCriticalThresholds(t *testing.T) {
	t.Run("db dumps present but none imported is critical", func(t *testing.T) {
		if !dbDumpsAllSkipped(1, 0) {
			t.Error("1 dump, 0 created should be critical")
		}
		if dbDumpsAllSkipped(0, 0) {
			t.Error("no dumps is not a failure")
		}
		if dbDumpsAllSkipped(2, 2) {
			t.Error("all imported is not a failure")
		}
	})

	t.Run("mail found but none pushed is critical", func(t *testing.T) {
		if !mailFoundNonePushed(1, 0) {
			t.Error("1 found, 0 pushed should be critical")
		}
		if mailFoundNonePushed(0, 0) {
			t.Error("no messages is not a failure")
		}
		if mailFoundNonePushed(3, 3) {
			t.Error("all pushed is not a failure")
		}
	})

	t.Run("zero healthy domains is critical", func(t *testing.T) {
		if !healthNoneHealthy(2, 0) {
			t.Error("2 probed, 0 healthy should be critical")
		}
		if healthNoneHealthy(0, 0) {
			t.Error("no domains probed is not a failure")
		}
		if healthNoneHealthy(2, 1) {
			t.Error("at least one healthy is not a failure")
		}
	})

	t.Run("any 5xx (crashing app) is critical even when others are healthy", func(t *testing.T) {
		if !healthAnyServerError(1) {
			t.Error("1 domain returning 5xx should be critical (JAB-40)")
		}
		if healthAnyServerError(0) {
			t.Error("no 5xx is not a failure")
		}
	})
}

func TestShouldDowngradeToDegraded(t *testing.T) {
	if !shouldDowngradeToDegraded(models.MigrationStateDone, []string{"databases: 1 dump(s) present but 0 imported"}) {
		t.Error("done + criticals must downgrade to degraded")
	}
	if shouldDowngradeToDegraded(models.MigrationStateDone, nil) {
		t.Error("done + no criticals must stay done")
	}
	if shouldDowngradeToDegraded(models.MigrationStateFailed, []string{"x"}) {
		t.Error("a failed job must not be re-stamped degraded")
	}
}
