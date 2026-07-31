package repository_test

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/repository"
)

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }

// GH #429 / KI-2. A reporter saw `pull-source` dial 22 despite setting a custom
// SSH port. The code was traced correct and later proven correct on a box (a job
// with source_port=2222 dialled 2222), which left "the port never reached the
// row" as the remaining explanation. These tests pin the persistence half so a
// regression there is caught here rather than by a stranger's failed migration.
func TestMigrationJobRepo_PatchDraft_PersistsSourcePort(t *testing.T) {
	gdb, mock, raw := newMockDB(t)
	defer raw.Close()
	repo := repository.NewMigrationJobRepository(gdb)

	// source_port must appear in the SET clause when a port is supplied.
	const upd = "UPDATE `migration_jobs` SET .*`source_port`=\\?"

	mock.ExpectBegin()
	mock.ExpectExec(upd).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	require.NoError(t, repo.PatchDraft(context.Background(), "job1",
		strPtr("plesk.example.com"), strPtr("root"), nil, nil, intPtr(8443)))

	require.NoError(t, mock.ExpectationsWereMet())
}

// A nil port means "not supplied by this PATCH" and must leave the column
// alone. If it ever started writing a zero instead, every Connection-step save
// that omitted the field would silently reset the job to port 22 — which is
// exactly the symptom reported in GH #429.
func TestMigrationJobRepo_PatchDraft_NilPortLeavesColumnAlone(t *testing.T) {
	gdb, mock, raw := newMockDB(t)
	defer raw.Close()
	repo := repository.NewMigrationJobRepository(gdb)

	mock.ExpectBegin()
	// No source_port in the SET clause at all.
	mock.ExpectExec("UPDATE `migration_jobs` SET `source_host`=\\?,`updated_at`=\\?").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	require.NoError(t, repo.PatchDraft(context.Background(), "job1",
		strPtr("plesk.example.com"), nil, nil, nil, nil))

	require.NoError(t, mock.ExpectationsWereMet())
}

// PatchDraft is deliberately gated on state='draft'. That means a Connection-step
// save arriving after the job has left draft updates NOTHING and reports no
// error — the port silently stays at whatever it was. This pins that the WHERE
// clause carries the state gate, so the behaviour is a documented decision
// rather than something a future edit erases by accident.
//
// It is also the most plausible remaining route to the GH #429 symptom on a
// current build: a port set too late is a port that was never stored.
func TestMigrationJobRepo_PatchDraft_IsGatedOnDraftState(t *testing.T) {
	gdb, mock, raw := newMockDB(t)
	defer raw.Close()
	repo := repository.NewMigrationJobRepository(gdb)

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `migration_jobs` SET .*WHERE id = \\? AND state = \\?").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	require.NoError(t, repo.PatchDraft(context.Background(), "job1",
		nil, nil, nil, nil, intPtr(2222)))

	require.NoError(t, mock.ExpectationsWereMet())
}
