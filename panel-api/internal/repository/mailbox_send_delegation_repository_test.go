package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
)

func TestSendDelegationCreate(t *testing.T) {
	db, mock, raw := newMockDB(t)
	defer raw.Close()

	repo := NewMailboxSendDelegationRepository(db)
	d := &models.MailboxSendDelegation{
		ID:                "01HZSENDAS000000000000001A",
		DelegateMailboxID: "01HZDELEG00000000000000001",
		GrantorMailboxID:  "01HZGRANT00000000000000001",
	}
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `mailbox_send_delegations`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	require.NoError(t, repo.Create(context.Background(), d))
	require.False(t, d.CreatedAt.IsZero(), "Create must stamp CreatedAt")
	require.False(t, d.UpdatedAt.IsZero(), "Create must stamp UpdatedAt")
	require.NoError(t, mock.ExpectationsWereMet())
}

// ListAllPairs joins mailboxes twice and drops any pair touching a disabled
// mailbox — this is the converger's authoritative input.
func TestSendDelegationListAllPairs(t *testing.T) {
	db, mock, raw := newMockDB(t)
	defer raw.Close()

	repo := NewMailboxSendDelegationRepository(db)
	mock.ExpectQuery("SELECT dm.email_cached AS delegate_email.*FROM `mailbox_send_delegations`.*JOIN mailboxes dm.*is_disabled = 0.*JOIN mailboxes gm.*is_disabled = 0").
		WillReturnRows(sqlmock.NewRows([]string{"delegate_email", "grantor_email"}).
			AddRow("support@x.test", "billing@x.test").
			AddRow("support@x.test", "sales@x.test"))

	pairs, err := repo.ListAllPairs(context.Background())
	require.NoError(t, err)
	require.Len(t, pairs, 2)
	require.Equal(t, "support@x.test", pairs[0].DelegateEmail)
	require.Equal(t, "billing@x.test", pairs[0].GrantorEmail)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSendDelegationDeleteByPair_NotFound(t *testing.T) {
	db, mock, raw := newMockDB(t)
	defer raw.Close()

	repo := NewMailboxSendDelegationRepository(db)
	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM `mailbox_send_delegations`").
		WillReturnResult(sqlmock.NewResult(0, 0)) // nothing deleted
	mock.ExpectCommit()

	err := repo.DeleteByPair(context.Background(), "d", "g")
	require.ErrorIs(t, err, ErrNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSendDelegationExistsPair(t *testing.T) {
	db, mock, raw := newMockDB(t)
	defer raw.Close()

	repo := NewMailboxSendDelegationRepository(db)
	mock.ExpectQuery("SELECT count.*FROM `mailbox_send_delegations` WHERE delegate_mailbox_id = \\? AND grantor_mailbox_id = \\?").
		WithArgs("d", "g").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	ok, err := repo.ExistsPair(context.Background(), "d", "g")
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, mock.ExpectationsWereMet())
}
