package cpanel

import (
	"strings"
	"testing"
)

// GH #723 follow-up. The original report was answered with a caveat:
//
//	"your itFlow site uses config.php (not a wp-config.php-style file), which
//	 jabali doesn't auto-rewrite yet — after migrating you may need to update
//	 its DB name/user/password by hand."
//
// That gap is worth closing rather than documenting: jabali ships an ITFlow
// installer, and jabali always namespaces databases to <account>_<name>, so a
// migrated ITFlow site is left pointing at a DB that no longer exists — the
// site simply doesn't load.
//
// The shape below is what ITFlow's own scripts/setup_cli.php emits at the
// commit we pin (itflowMasterPinnedCommit): var_export()'d single-quoted
// values followed by the mysqli_connect line.
const itflowConfigPHP = `<?php
$dbhost = 'localhost';
$dbusername = 'notary_itflow';
$dbpassword = 'sourcepw123';
$database = 'notary_itflow';
$mysqli = mysqli_connect($dbhost, $dbusername, $dbpassword, $database) or die('Database Connection Failed');
`

// Same multi-database account as the GH #723 WordPress test — johnnyq's real
// HestiaCP backup carries BOTH notary_45635 (WordPress) and notary_itflow.
// Credentials are keyed by SOURCE db name, per the #723 fix.
func itflowCreds() map[string]DBCredential {
	return map[string]DBCredential{
		"notary_45635":  {DBName: "johnnyq_45635", DBUser: "johnnyq_45635", Password: "PANELPW_45635"},
		"notary_itflow": {DBName: "johnnyq_itflow", DBUser: "johnnyq_itflow", Password: "PANELPW_ITFLOW"},
	}
}

func TestRewriteITFlow_MultiDBPicksItsOwnCredential(t *testing.T) {
	out, changed := rewriteITFlow(itflowConfigPHP, itflowCreds())
	if !changed {
		t.Fatal("itflow config.php was NOT rewritten")
	}
	for _, want := range []string{
		"$database = 'johnnyq_itflow';",
		"$dbusername = 'johnnyq_itflow';",
		"$dbpassword = 'PANELPW_ITFLOW';",
		"$dbhost = 'localhost';",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q\n--- got ---\n%s", want, out)
		}
	}
	// It must not have grabbed the WordPress credential from the same account.
	if strings.Contains(out, "johnnyq_45635") || strings.Contains(out, "PANELPW_45635") {
		t.Errorf("picked the WordPress credential instead of its own\n--- got ---\n%s", out)
	}
	// The connect line is what keys detection — it must survive intact.
	if !strings.Contains(out, "mysqli_connect($dbhost, $dbusername, $dbpassword, $database)") {
		t.Error("the mysqli_connect line was mangled")
	}
}

// The safety-critical case. config.php is one of the most common filenames in
// PHP hosting, and this rewriter is pointed at <docroot>/config.php on every
// migrated site. Touching a file that merely happens to be named config.php
// would corrupt an unrelated app, so detection keys on ITFlow's own
// mysqli_connect signature rather than the filename.
func TestRewriteITFlow_IgnoresForeignConfigPHP(t *testing.T) {
	foreign := []struct{ name, body string }{
		{"unrelated app with similar var names", `<?php
$dbhost = 'localhost';
$dbusername = 'someapp';
$dbpassword = 'secret';
$database = 'someapp_db';
$pdo = new PDO("mysql:host=$dbhost;dbname=$database", $dbusername, $dbpassword);
`},
		{"generic settings file", `<?php
return ['debug' => true, 'name' => 'My App'];
`},
		{"php file with no db config at all", "<?php\n// nothing here\n"},
		{"empty", ""},
	}
	for _, f := range foreign {
		t.Run(f.name, func(t *testing.T) {
			out, changed := rewriteITFlow(f.body, itflowCreds())
			if changed {
				t.Errorf("rewrote a NON-ITFlow config.php — this corrupts unrelated apps\n--- got ---\n%s", out)
			}
			if out != f.body {
				t.Error("body was modified despite changed=false")
			}
		})
	}
}

// Single-database accounts have nothing to match against, so the sole
// credential is used — same fallback the WordPress and Joomla rewriters apply.
func TestRewriteITFlow_SingleCredentialFallback(t *testing.T) {
	// Source DB name deliberately absent from the map.
	creds := map[string]DBCredential{
		"something_else": {DBName: "acct_itflow", DBUser: "acct_itflow", Password: "PW"},
	}
	out, changed := rewriteITFlow(itflowConfigPHP, creds)
	if !changed {
		t.Fatal("single-credential fallback did not fire")
	}
	if !strings.Contains(out, "$database = 'acct_itflow';") {
		t.Errorf("fallback credential not applied\n--- got ---\n%s", out)
	}
}

// With several credentials and no name match there is no safe choice, so the
// file must be left alone rather than guessed at.
func TestRewriteITFlow_NoMatchMultiCredentialLeavesFileAlone(t *testing.T) {
	creds := map[string]DBCredential{
		"other_a": {DBName: "a", DBUser: "a", Password: "x"},
		"other_b": {DBName: "b", DBUser: "b", Password: "y"},
	}
	out, changed := rewriteITFlow(itflowConfigPHP, creds)
	if changed {
		t.Errorf("guessed a credential with no name match\n--- got ---\n%s", out)
	}
	if out != itflowConfigPHP {
		t.Error("body modified despite changed=false")
	}
}

// A password containing a quote must not break out of the generated PHP
// string literal — phpEscape handles it, same as the other rewriters.
func TestRewriteITFlow_EscapesQuotesInPassword(t *testing.T) {
	creds := map[string]DBCredential{
		"notary_itflow": {DBName: "d", DBUser: "u", Password: `pa'ss\x`},
	}
	out, changed := rewriteITFlow(itflowConfigPHP, creds)
	if !changed {
		t.Fatal("not rewritten")
	}
	if strings.Contains(out, `$dbpassword = 'pa'ss`) {
		t.Errorf("unescaped quote broke the PHP literal\n--- got ---\n%s", out)
	}
}
