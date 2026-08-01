package cpanel

import (
	"strings"
	"testing"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/migrate"
)

// JAB-207. A cPanel restore with --preserve-source-state left every migrated
// WordPress serving "Error establishing a database connection" while the
// summary reported success.
//
// Two steps own the same MySQL account whenever the source DB user is spelled
// the same as the panel-managed one — the norm when the destination account
// keeps the source's name (vidoflexco_db -> vidoflexco_db):
//
//  1. the dump loop creates it with a fresh TEMP password and hands that to
//     the app-config rewriter, which writes it into wp-config.php;
//  2. the compat pass then calls db_user.create with the source hash, whose
//     unconditional ALTER USER (deliberate — GH #633, the preserved hash must
//     win) replaces that temp password.
//
// The account then authenticates with the ORIGINAL password while wp-config
// holds the temp one. Neither step is wrong on its own, which is why the
// summary was green.
//
// The source config already carries the original password and it matched that
// hash on the source, so the fix keeps it: rewrite DB name/user/host, leave the
// password untouched.
func TestRewriteWordPress_KeepsSourcePasswordWhenCompatUserOwnsAccount(t *testing.T) {
	t.Parallel()

	const wpConfig = `<?php
define('DB_NAME', 'vidoflexco_db');
define('DB_USER', 'vidoflexco_db');
define('DB_PASSWORD', 'origPass14chr');
define('DB_HOST', 'localhost');
`

	creds := map[string]DBCredential{
		"vidoflexco_db": {
			DBName:   "vidoflexco_db",
			DBUser:   "vidoflexco_db",
			Password: "TEMPgenerated26charvalue00",
			// Set by the compat pass on a name collision.
			KeepSourcePassword: true,
		},
	}

	out, changed := rewriteWordPress(wpConfig, creds)

	if strings.Contains(out, "TEMPgenerated26charvalue00") {
		t.Errorf("wp-config was rewritten to the temp password, which the compat "+
			"ALTER USER has already invalidated — this is the exact failure "+
			"JAB-207 reported:\n%s", out)
	}
	if !strings.Contains(out, "define('DB_PASSWORD', 'origPass14chr');") {
		t.Errorf("the original password must survive — it is what the preserved "+
			"hash authenticates:\n%s", out)
	}
	// The rest still has to be rewritten; only the password is special.
	if !strings.Contains(out, "define('DB_HOST', 'localhost');") {
		t.Errorf("DB_HOST should still be normalised:\n%s", out)
	}
	_ = changed
}

// The default path must be unaffected: with no collision the rewriter still
// publishes the panel-managed temp password, which is what the freshly created
// user authenticates with.
func TestRewriteWordPress_WritesTempPasswordWhenNoCollision(t *testing.T) {
	t.Parallel()

	const wpConfig = `<?php
define('DB_NAME', 'src_db');
define('DB_USER', 'src_user');
define('DB_PASSWORD', 'origPass');
define('DB_HOST', 'localhost');
`
	creds := map[string]DBCredential{
		"src_db": {DBName: "tgt_db", DBUser: "tgt_db", Password: "tempPW"},
	}

	out, _ := rewriteWordPress(wpConfig, creds)

	for _, want := range []string{
		"define('DB_NAME', 'tgt_db');",
		"define('DB_USER', 'tgt_db');",
		"define('DB_PASSWORD', 'tempPW');",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q — the non-preserve path must still rewrite all "+
				"three:\n%s", want, out)
		}
	}
}

// The shared rewriter is used by the wordpress_ssh import too, so its nil
// contract is pinned directly rather than only through the cPanel caller.
func TestRewriteWPConfigDBFields_NilPasswordLeavesItAlone(t *testing.T) {
	t.Parallel()

	const in = `<?php
define('DB_NAME', 'a');
define('DB_USER', 'b');
define('DB_PASSWORD', 'keepme');
define('DB_HOST', 'oldhost');
`
	out, changed := migrate.RewriteWPConfigDBFields(in, "newdb", "newuser", nil, "localhost")

	if !strings.Contains(out, "define('DB_PASSWORD', 'keepme');") {
		t.Errorf("nil password must leave DB_PASSWORD untouched:\n%s", out)
	}
	if !strings.Contains(out, "define('DB_NAME', 'newdb');") ||
		!strings.Contains(out, "define('DB_USER', 'newuser');") ||
		!strings.Contains(out, "define('DB_HOST', 'localhost');") {
		t.Errorf("the other three constants must still be rewritten:\n%s", out)
	}
	if !changed {
		t.Error("changed should be true — name/user/host did change")
	}
}

// A password that happens to contain a quote must still round-trip through the
// escaper on the normal path; nil must not accidentally disable escaping for
// everyone.
func TestRewriteWPConfigDBFields_EscapesWhenPasswordSupplied(t *testing.T) {
	t.Parallel()

	const in = `<?php
define('DB_NAME', 'a');
define('DB_USER', 'b');
define('DB_PASSWORD', 'x');
define('DB_HOST', 'h');
`
	pw := `pa'ss`
	out, _ := migrate.RewriteWPConfigDBFields(in, "d", "u", &pw, "localhost")
	if !strings.Contains(out, `define('DB_PASSWORD', 'pa\'ss');`) {
		t.Errorf("quote in the password was not escaped:\n%s", out)
	}
}
