package main

import (
	"testing"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
)

func strp(s string) *string { return &s }

// JAB-56: absent plan → import all; valid plan → honored; malformed → fail closed.
func TestMigrationPlanAreasChecked(t *testing.T) {
	// no plan → all true, ok
	if pa, ok := migrationPlanAreasChecked(&models.MigrationJob{}); !ok || !pa.Databases || !pa.Websites {
		t.Errorf("absent plan should import all: %+v ok=%v", pa, ok)
	}
	// valid plan with databases off
	j := &models.MigrationJob{PlanJSON: strp(`{"areas":{"websites":true,"databases":false,"mailboxes":true,"dns":true,"ssl":true,"cron":true}}`)}
	pa, ok := migrationPlanAreasChecked(j)
	if !ok || pa.Databases || !pa.Websites {
		t.Errorf("databases should be off, websites on: %+v ok=%v", pa, ok)
	}
	// malformed → fail closed (ok=false)
	if _, ok := migrationPlanAreasChecked(&models.MigrationJob{PlanJSON: strp(`{not json`)}); ok {
		t.Error("malformed plan must fail closed (ok=false)")
	}
}
