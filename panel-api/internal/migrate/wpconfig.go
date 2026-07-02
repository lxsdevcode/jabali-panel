package migrate

import (
	"fmt"
	"regexp"
	"strings"
)

// wpConfigDefineRe matches a wp-config.php `define('DB_KEY', 'value');` for the
// four DB constants, tolerating single OR double quotes and flexible spacing.
// Identical to the cPanel restore's proven pattern — this is the single shared
// copy so the cPanel and wordpress_ssh (GH #647/#648) imports never drift.
var wpConfigDefineRe = regexp.MustCompile(`(?m)^\s*define\(\s*['"](DB_NAME|DB_USER|DB_PASSWORD|DB_HOST)['"]\s*,\s*['"]([^'"]*)['"]\s*\)\s*;`)

// phpSingleQuoteEscape escapes a value for a PHP single-quoted string literal:
// backslash then single-quote (PHP only special-cases \ and ' inside '...').
func phpSingleQuoteEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	return s
}

// RewriteWPConfigDB rewrites the DB_NAME/DB_USER/DB_PASSWORD/DB_HOST define()s
// in a wp-config.php body to the destination Jabali credentials, returning the
// new body and whether anything changed. It is PURE (no I/O) — the caller reads
// the file and writes it back (via the agent), exactly like the cPanel restore.
// Shared by the cPanel restore and the wordpress_ssh import so both apply the
// identical, proven rewrite. Values are PHP single-quote escaped.
func RewriteWPConfigDB(text, dbName, dbUser, dbPass, dbHost string) (string, bool) {
	if !strings.Contains(text, "DB_NAME") {
		return text, false
	}
	out := wpConfigDefineRe.ReplaceAllStringFunc(text, func(line string) string {
		m := wpConfigDefineRe.FindStringSubmatch(line)
		switch m[1] {
		case "DB_NAME":
			return fmt.Sprintf("define('DB_NAME', '%s');", phpSingleQuoteEscape(dbName))
		case "DB_USER":
			return fmt.Sprintf("define('DB_USER', '%s');", phpSingleQuoteEscape(dbUser))
		case "DB_PASSWORD":
			return fmt.Sprintf("define('DB_PASSWORD', '%s');", phpSingleQuoteEscape(dbPass))
		case "DB_HOST":
			return fmt.Sprintf("define('DB_HOST', '%s');", phpSingleQuoteEscape(dbHost))
		}
		return line
	})
	return out, out != text
}
