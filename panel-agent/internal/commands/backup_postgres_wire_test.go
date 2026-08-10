package commands

// GH #1015: every account backup silently omitted its Postgres databases.
//
// The panel has sent `databases_postgres` on backup.create since M37
// Phase 2, and both the backup.databases sub-command and the .pgdump
// restore path handled Postgres — but backupCreateParams never declared
// the field, so encoding/json dropped the key on the floor and the
// orchestrator ran the db stage with the MariaDB list only. The job then
// sealed `succeeded`, which is what made the omission invisible: nothing
// failed, the data just wasn't there.
//
// These tests pin the wire contract at the exact layer it broke.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"git.jabali-panel.com/shukivaknin/jabali2/internal/backup"
)

// The decode itself — the bug was a missing struct field, which is
// invisible to the compiler and to every test that constructs the params
// in Go rather than from JSON.
func TestBackupCreateParams_DecodesDatabasesPostgres(t *testing.T) {
	body := `{"job_id":"` + testJobID + `","user_id":"` + testUserID + `",` +
		`"username":"alice","databases":["alice_shop"],"databases_postgres":["alice_pg"]}`
	var req backupCreateParams
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatal(err)
	}
	if len(req.DatabasesPostgres) != 1 || req.DatabasesPostgres[0] != "alice_pg" {
		t.Fatalf("databases_postgres did not decode: %#v — the agent is dropping the panel's Postgres list again", req.DatabasesPostgres)
	}
}

// An account whose ONLY database is Postgres must run the db stage, not
// skip it. Pre-fix this returned a single skipped stage with "no
// databases" — the exact silent omission from the issue. We don't need a
// restic repo to see the difference: attempting the stage without restic
// fails loudly, and failed ≠ skipped is the whole point.
func TestRunDatabaseStage_PostgresOnlyIsNotSkipped(t *testing.T) {
	got := runDatabaseStage(context.Background(), backupCreateParams{
		JobID: testJobID, UserID: testUserID, Username: "alice",
		DatabasesPostgres: []string{"alice_pg"},
	})
	if len(got) == 1 && got[0].Status == backup.StageStatusSkipped {
		t.Fatalf("a Postgres-only account was skipped as %q — its databases are silently omitted from the backup",
			strings.Join(got[0].Warnings, "; "))
	}
}

// The mixed case must forward BOTH lists to the sub-command. The
// sub-command's own params struct is the boundary: marshal what
// runDatabaseStage builds and prove the Postgres list survives the hop.
func TestRunDatabaseStage_ForwardsPostgresList(t *testing.T) {
	req := backupCreateParams{
		JobID: testJobID, UserID: testUserID, Username: "alice",
		Databases:         []string{"alice_shop"},
		DatabasesPostgres: []string{"alice_pg", "alice_analytics"},
	}
	body, _ := json.Marshal(backupDatabasesParams{
		JobID: req.JobID, UserID: req.UserID, Username: req.Username,
		Databases: req.Databases, DatabasesPostgres: req.DatabasesPostgres,
	})
	var round backupDatabasesParams
	if err := json.Unmarshal(body, &round); err != nil {
		t.Fatal(err)
	}
	if len(round.DatabasesPostgres) != 2 {
		t.Fatalf("Postgres list lost between orchestrator and sub-command: %#v", round.DatabasesPostgres)
	}
}
