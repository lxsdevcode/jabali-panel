package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/repository"
)

func mailJoinRows(now time.Time) *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "domain_id", "domain_name", "user_id", "user_username",
		"status", "issued_at", "expires_at", "last_error", "last_attempt_at",
	}).AddRow(
		"mail-cert:01ARZ3NDEKTSV4RRFFQ69G5FAV",
		"01ARZ3NDEKTSV4RRFFQ69G5FAV",
		"mail.example.com",
		"01ARZ3NDEKTSV4RRFFQ69G5F00",
		"alice",
		"failed",
		nil, nil,
		mailStrPtr("certbot issue failed: rate limited"),
		now,
	)
}

func TestMailCert_ListWithDomain(t *testing.T) {
	t.Parallel()
	gdb, mock, raw := newMockDB(t)
	defer raw.Close()
	repo := repository.NewMailCertificateRepository(gdb)

	mock.ExpectQuery("FROM mail_certificate mc JOIN domains d ON mc.domain_id = d.id JOIN users u").
		WillReturnRows(mailJoinRows(time.Now()))

	rows, err := repo.ListWithDomain(context.Background())
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "mail-cert:01ARZ3NDEKTSV4RRFFQ69G5FAV", rows[0].ID)
	assert.Equal(t, "mail.example.com", rows[0].DomainName)
	assert.Equal(t, "alice", rows[0].UserUsername)
	assert.Equal(t, "failed", rows[0].Status)
	require.NotNil(t, rows[0].LastError)
	assert.Contains(t, *rows[0].LastError, "rate limited")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMailCert_ListWithDomainByUser(t *testing.T) {
	t.Parallel()
	gdb, mock, raw := newMockDB(t)
	defer raw.Close()
	repo := repository.NewMailCertificateRepository(gdb)

	mock.ExpectQuery("FROM mail_certificate mc JOIN domains").
		WithArgs("01ARZ3NDEKTSV4RRFFQ69G5F00", "disabled").
		WillReturnRows(mailJoinRows(time.Now()))

	rows, err := repo.ListWithDomainByUser(context.Background(), "01ARZ3NDEKTSV4RRFFQ69G5F00")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "mail.example.com", rows[0].DomainName)
	require.NoError(t, mock.ExpectationsWereMet())
}

func mailStrPtr(s string) *string { return &s }
