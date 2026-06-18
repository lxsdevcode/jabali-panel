package repository

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/models"
)

// TestDomainCreate_EmailDisabledIncludedInInsert is the GH #216 regression
// gate. A "No Mail" domain (mail_provider none → DeriveMailFlags returns
// EmailEnabled=false) must persist email_enabled=0. With the old
// `gorm:"...;default:1"` tag, GORM substituted the DB default for the
// Go zero value and OMITTED the column from the INSERT entirely, so the
// row landed email_enabled=1 and the reconciler backfilled mail cert+DNS.
//
// GORM doesn't omit a defaulted zero-value column — it MUTATES the struct
// field to the default before building VALUES (per the cron default:1
// scar). So the gate is: after building the Create, EmailEnabled must
// still be false. With the `default:1` tag restored GORM flips it to true
// here, which is exactly how the false value became email_enabled=1 on the
// wire.
func TestDomainCreate_EmailDisabledStaysFalse(t *testing.T) {
	db, _, raw := newMockDB(t)
	defer raw.Close()

	dom := &models.Domain{
		ID:           "01TESTDOMAIN0000000000000A",
		UserID:       "01TESTUSER00000000000000A",
		Name:         "nomail.example.com",
		DocRoot:      "/home/u/domains/nomail.example.com/public_html",
		MailProvider: models.MailProviderNone,
		EmailEnabled: false, // DeriveMailFlags("none")
		SkipAutoSAN:  true,
	}

	tx := db.Session(&gorm.Session{DryRun: true, SkipDefaultTransaction: true}).Create(dom)
	require.NoError(t, tx.Error)

	require.False(t, dom.EmailEnabled,
		"GORM must not flip EmailEnabled=false → true on insert (GH #216: the `default:1` tag did exactly that, persisting email_enabled=1 for No-Mail domains)")
}
