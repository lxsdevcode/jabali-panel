package repository

import (
	"context"
	"database/sql"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newMockSharedResDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, *sql.DB) {
	t.Helper()
	raw, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT VERSION()")).
		WillReturnRows(sqlmock.NewRows([]string{"VERSION()"}).AddRow("10.11.6-MariaDB"))
	gdb, err := gorm.Open(mysql.New(mysql.Config{Conn: raw, SkipInitializeWithVersion: false}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	return gdb, mock, raw
}

func TestSharedRes_ExistsByEmail(t *testing.T) {
	db, mock, raw := newMockSharedResDB(t)
	defer raw.Close()
	repo := NewSharedResourceRepository(db)

	mock.ExpectQuery("SELECT count.* FROM .shared_resources. WHERE email_cached = .").
		WithArgs("team@example.org").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	exists, err := repo.ExistsByEmail(context.Background(), "team@example.org")
	require.NoError(t, err)
	require.True(t, exists)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSharedRes_ListByDomainAndKind(t *testing.T) {
	db, mock, raw := newMockSharedResDB(t)
	defer raw.Close()
	repo := NewSharedResourceRepository(db)

	mock.ExpectQuery("SELECT .* FROM .shared_resources. WHERE domain_id = . AND kind = .").
		WithArgs("dom1", "calendar").
		WillReturnRows(sqlmock.NewRows([]string{"id", "domain_id", "kind", "display_name"}).
			AddRow("res1", "dom1", "calendar", "Team Cal"))

	rows, err := repo.ListByDomainAndKind(context.Background(), "dom1", "calendar")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "calendar", rows[0].Kind)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSharedRes_UpdateProjection_NotFound(t *testing.T) {
	db, mock, raw := newMockSharedResDB(t)
	defer raw.Close()
	repo := NewSharedResourceRepository(db)

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE .shared_resources. SET").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	err := repo.UpdateProjection(context.Background(), "missing", "acct1", "col1")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestSharedRes_ReplaceGrants_Empty(t *testing.T) {
	db, mock, raw := newMockSharedResDB(t)
	defer raw.Close()
	repo := NewSharedResourceRepository(db)

	// Empty grants = delete-all then no insert.
	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM .shared_resource_grants. WHERE resource_id = .").
		WithArgs("res1").
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()

	err := repo.ReplaceGrants(context.Background(), "res1", nil)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
