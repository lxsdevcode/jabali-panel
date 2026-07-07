package repository_test

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/repository"
)

// JAB-38 / --retry-from-scratch: DeleteStagesForJob wipes every migration_stages
// row for a job so the runner re-seeds + re-runs the full pipeline.
func TestMigrationJobRepo_DeleteStagesForJob(t *testing.T) {
	gdb, mock, raw := newMockDB(t)
	defer raw.Close()
	repo := repository.NewMigrationJobRepository(gdb)

	const del = "DELETE FROM `migration_stages` WHERE job_id = \\?"
	mock.ExpectBegin()
	mock.ExpectExec(del).WithArgs("job1").WillReturnResult(sqlmock.NewResult(0, 4))
	mock.ExpectCommit()

	require.NoError(t, repo.DeleteStagesForJob(context.Background(), "job1"))
	require.NoError(t, mock.ExpectationsWereMet())
}
