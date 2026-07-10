package main

import (
	"testing"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
)

// GH #327: the online HestiaCP flow failed at `stage analyze: connect:` with
// "[none password]" because migrationSSHUser returned job.SourceUser (the
// account being migrated, e.g. "itflowapp") as the SSH login. HestiaCP accounts
// have no SSH shell and its v-* commands need root, so the analyze stage must
// dial as root — matching discovery/pull, which already do.
func TestMigrationSSHUser(t *testing.T) {
	cases := []struct {
		name string
		job  *models.MigrationJob
		want string
	}{
		{"nil job", nil, "root"},
		{"hestia account is NOT the ssh login -> root",
			&models.MigrationJob{SourceKind: models.MigrationSourceHestia, SourceUser: "itflowapp"}, "root"},
		{"hestia empty user -> root",
			&models.MigrationJob{SourceKind: models.MigrationSourceHestia, SourceUser: ""}, "root"},
		{"cpanel account IS the ssh login",
			&models.MigrationJob{SourceKind: models.MigrationSourceCpanel, SourceUser: "acct1"}, "acct1"},
		{"directadmin uses the (pivoted) source user",
			&models.MigrationJob{SourceKind: models.MigrationSourceDirectAdmin, SourceUser: "admin"}, "admin"},
		{"empty source user falls back to root",
			&models.MigrationJob{SourceKind: models.MigrationSourceCpanel, SourceUser: ""}, "root"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := migrationSSHUser(tc.job); got != tc.want {
				t.Errorf("migrationSSHUser = %q, want %q", got, tc.want)
			}
		})
	}
}
