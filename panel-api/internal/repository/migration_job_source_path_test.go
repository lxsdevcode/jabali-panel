package repository_test

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/repository"
)

// GH #647 S1: UpdateSourcePath is a dedicated setter (not an allowlist patch),
// so the WP root can never be silently dropped. Verify it issues an UPDATE of
// source_path and maps RowsAffected=0 → ErrNotFound.
func TestMigrationJobRepo_UpdateSourcePath(t *testing.T) {
	gdb, mock, raw := newMockDB(t)
	defer raw.Close()
	repo := repository.NewMigrationJobRepository(gdb)

	const upd = "UPDATE `migration_jobs` SET `source_path`=\\?,`updated_at`=\\? WHERE id = \\?"

	// found → nil
	mock.ExpectBegin()
	mock.ExpectExec(upd).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	require.NoError(t, repo.UpdateSourcePath(context.Background(), "job1", "/home/master/applications/abc/public_html"))

	// missing → ErrNotFound
	mock.ExpectBegin()
	mock.ExpectExec(upd).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	err := repo.UpdateSourcePath(context.Background(), "nope", "/x")
	require.ErrorIs(t, err, repository.ErrNotFound)

	require.NoError(t, mock.ExpectationsWereMet())
}
