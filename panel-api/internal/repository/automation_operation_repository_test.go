package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
)

func TestAutomationOperationCreate(t *testing.T) {
	db, mock, raw := newMockDB(t)
	defer raw.Close()
	repo := NewAutomationOperationRepository(db)
	op := &models.AutomationOperation{ID: "01AOP00000000000000000001A", TokenID: "01TOK0000000000000000001AA", Action: "backup"}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `automation_operations`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	require.NoError(t, repo.Create(context.Background(), op))
	require.Equal(t, "pending", op.Status, "Create defaults status to pending")
	require.False(t, op.CreatedAt.IsZero())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAutomationOperationFindByIdempotencyKey(t *testing.T) {
	db, mock, raw := newMockDB(t)
	defer raw.Close()
	repo := NewAutomationOperationRepository(db)

	mock.ExpectQuery("SELECT .* FROM `automation_operations` WHERE token_id = \\? AND idempotency_key = \\?").
		WithArgs("tok1", "idem-abc", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "token_id", "action", "status"}).
			AddRow("01AOP00000000000000000001A", "tok1", "backup", "running"))

	op, err := repo.FindByIdempotencyKey(context.Background(), "tok1", "idem-abc")
	require.NoError(t, err)
	require.Equal(t, "running", op.Status)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAutomationOperationSetStatus_NotFound(t *testing.T) {
	db, mock, raw := newMockDB(t)
	defer raw.Close()
	repo := NewAutomationOperationRepository(db)

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `automation_operations` SET").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	err := repo.SetStatus(context.Background(), "missing", "done", "ok")
	require.ErrorIs(t, err, ErrNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}
