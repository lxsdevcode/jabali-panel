package commands

import (
	"os"
	"path/filepath"

	"git.jabali-panel.com/shukivaknin/jabali2/internal/filesafe"
)

// sensitiveBasenames hold tenant secrets (DB credentials, app keys, cache
// passwords) that another tenant must NOT be able to read across the shared
// www-data group via an un-sandboxed cron-php (Gitea #430 interim). They get
// 0600 owner-only: FPM and the owner's own wp-cli read them AS the owner; nginx
// never reads .php; another tenant's process (uid mismatch, no group/other bit)
// gets EACCES. The structural fix is per-user groups — see
// plans/m430-tenant-isolation-acl.md.
var sensitiveBasenames = map[string]bool{
	"wp-config.php": true,
	".env":          true,
}

// sensitiveFileMode returns 0600 for a secret-bearing file, else def. Callers
// stamping file modes (files.write, files.ingest) route through this so a
// wp-config.php / .env written or replaced via the file manager stays owner-only
// instead of the default 0640/0644 group-readable.
func sensitiveFileMode(pathOrName string, def os.FileMode) os.FileMode {
	if sensitiveBasenames[filepath.Base(pathOrName)] {
		return 0o600
	}
	return def
}

// hardenSensitiveFilesInScope re-tightens the docroot-level wp-config.php / .env
// to 0600 via the escape-proof filesafe primitive (refuses symlinks, can't be
// redirected out of the tree). Self-healing: called per reconcile from
// domain.create so it backfills pre-existing installs and re-tightens any file
// the file manager re-widened. Best-effort — never fails the caller.
func hardenSensitiveFilesInScope(username, docRoot string) {
	if username == "" || docRoot == "" {
		return
	}
	clean := filepath.Clean(docRoot)
	scope, err := filesafe.NewScope("", username, []string{clean})
	if err != nil {
		return
	}
	for name := range sensitiveBasenames {
		p := filepath.Join(clean, name)
		if fi, lerr := os.Lstat(p); lerr == nil && fi.Mode().IsRegular() {
			_ = scope.ChmodInScope(p, 0o600)
		}
	}
}
