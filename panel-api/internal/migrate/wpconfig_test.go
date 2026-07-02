package migrate

import "testing"

func TestRewriteWPConfigDB(t *testing.T) {
	// Both quote styles + varied spacing + a password with a quote/backslash.
	src := `<?php
define('DB_NAME', 'old_db');
define( "DB_USER" , "old_user" );
define('DB_PASSWORD', 'oldpass');
define('DB_HOST', 'remote:3306');
$table_prefix = 'wp_';
`
	out, changed := RewriteWPConfigDB(src, "jab_db", "jab_user", `p'a\ss`, "localhost")
	if !changed {
		t.Fatal("expected changed=true")
	}
	for _, want := range []string{
		"define('DB_NAME', 'jab_db');",
		"define('DB_USER', 'jab_user');",
		`define('DB_PASSWORD', 'p\'a\\ss');`, // ' and \ escaped for PHP single-quote
		"define('DB_HOST', 'localhost');",
	} {
		if !contains(out, want) {
			t.Errorf("output missing %q\n---\n%s", want, out)
		}
	}
	// No DB_NAME → no change.
	if _, ch := RewriteWPConfigDB("<?php // nothing", "a", "b", "c", "d"); ch {
		t.Error("expected no change when DB_NAME absent")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
