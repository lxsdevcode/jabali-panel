package commands

import "testing"

// TestRestoreDBNameRe pins the manifest database-name whitelist used before
// privileged restore SQL (Gitea #463): valid identifiers pass; names carrying
// quotes, backticks, whitespace, or SQL/shell metacharacters are rejected so a
// poisoned backup manifest cannot inject into psql/createdb/mariadb.
func TestRestoreDBNameRe(t *testing.T) {
	valid := []string{"wp", "site_db", "tenant-01", "A", "db_123-x"}
	for _, n := range valid {
		if !restoreDBNameRe.MatchString(n) {
			t.Errorf("valid db name %q should be accepted", n)
		}
	}
	invalid := []string{
		"",                       // empty
		"1db",                    // leading digit
		"db name",                // space
		"db';DROP TABLE x;--",    // SQL injection
		"db`whoami`",             // backtick
		"db$(touch /tmp/x)",      // shell
		"../etc",                 // path
		"db\nx",                  // newline
		"' OR '1'='1",            // quote
		"тест",                   // non-ascii
	}
	for _, n := range invalid {
		if restoreDBNameRe.MatchString(n) {
			t.Errorf("invalid db name %q must be rejected", n)
		}
	}
}
