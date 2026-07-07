package cpanel

import "testing"

// JAB-27: HestiaCP dumps (`<user>_<db>.mysql.sql`) must derive a valid,
// dot-free destination DB name instead of being skipped by the validator.
func TestDeriveDestDBName(t *testing.T) {
	cases := []struct {
		name       string
		dumpPath   string
		targetUser string
		wantFinal  string
		wantBase   string
		wantOK     bool
	}{
		{
			name:       "hestia mysql.sql (JAB-27 regression)",
			dumpPath:   "/var/lib/jabali-migrations/x/db/itflowapp_test/itflowapp_test.mysql.sql",
			targetUser: "itflowapp",
			wantFinal:  "itflowapp_test",
			wantBase:   "itflowapp_test",
			wantOK:     true,
		},
		{
			name:       "hestia pgsql.sql",
			dumpPath:   "/x/db/acme_shop/acme_shop.pgsql.sql",
			targetUser: "acme",
			wantFinal:  "acme_shop",
			wantBase:   "acme_shop",
			wantOK:     true,
		},
		{
			name:       "cpanel plain .sql still works",
			dumpPath:   "/x/cpuser_blogdb.sql",
			targetUser: "newuser",
			wantFinal:  "newuser_blogdb",
			wantBase:   "cpuser_blogdb",
			wantOK:     true,
		},
		{
			name:       "no underscore falls back to full base",
			dumpPath:   "/x/wordpress.mysql.sql",
			targetUser: "bob",
			wantFinal:  "bob_wordpress",
			wantBase:   "wordpress",
			wantOK:     true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			final, base, ok := deriveDestDBName(tc.dumpPath, tc.targetUser)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v (final=%q)", ok, tc.wantOK, final)
			}
			if final != tc.wantFinal {
				t.Errorf("finalName = %q, want %q", final, tc.wantFinal)
			}
			if base != tc.wantBase {
				t.Errorf("sourceBase = %q, want %q", base, tc.wantBase)
			}
		})
	}
}
