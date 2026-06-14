// php_cli_wrapper.go — per-user CLI PHP wrapper (GH #184).
//
// The panel pins each user's PHP version for their FPM pool (web), but the
// interactive shell, Composer, wp-cli, and cron resolve a bare `php` to the
// host default /usr/bin/php — so a user pinned to a non-default version got
// the wrong version at the CLI, and extensions enabled for their version
// looked "missing". This writes a per-user `php` that points at their pinned
// version; the SSH shell + cron prepend its dir to PATH so `php`, and
// anything with a `#!/usr/bin/env php` shebang (Composer, wp-cli), follow it.
//
// Layout (option B): /home/<user>/.jabali/bin/php -> /usr/bin/php<version>.
// The `.jabali` + `.jabali/bin` dirs are root-owned 0755 so a tenant can't
// quietly swap the wrapper for another binary. This is best-effort, not a
// security boundary: the tenant owns their home and could rename .jabali,
// but that only downgrades their OWN CLI — no cross-tenant or privilege
// impact. The symlink target is always a validated /usr/bin/php<version>
// that exists on disk; never a tenant-controlled path.
package commands

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// BackfillUserCLIPHP ensures a per-user CLI php wrapper for every user that
// already has a version pin, run once at agent startup. The reconciler only
// fires php.pool.apply for pending/error pools, so active pools (existing
// users) would otherwise never get a wrapper; a restart (e.g. `jabali
// update`) backfills them all. Best-effort: each user logged + skipped on
// error, never blocks boot.
func BackfillUserCLIPHP(log *slog.Logger) {
	root := os.Getenv("JABALI_PHP_VER_PIN_ROOT")
	if root == "" {
		root = "/etc/jabali-panel/user-phpver"
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return // no pins yet (fresh host) → nothing to backfill
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		username := e.Name()
		b, rerr := os.ReadFile(filepath.Join(root, username))
		if rerr != nil {
			continue
		}
		version := strings.TrimSpace(string(b))
		if err := ensureUserCLIPHP(username, version); err != nil && log != nil {
			log.Warn("backfill per-user CLI php wrapper", "user", username, "version", version, "err", err)
		}
	}
}

// userCLIPHPBinDir returns the per-user wrapper bin dir for a username.
func userCLIPHPBinDir(username string) string {
	return filepath.Join("/home", username, ".jabali", "bin")
}

// ensureUserCLIPHP writes/refreshes /home/<user>/.jabali/bin/php as a symlink
// to /usr/bin/php<version>. Idempotent: a symlink already pointing at the
// right target is left untouched (no churn on every reconciler tick). A user
// without a /home/<user> directory (system/service account) is a clean no-op.
func ensureUserCLIPHP(username, version string) error {
	if !phpPoolUsernameRegex.MatchString(username) {
		return fmt.Errorf("ensureUserCLIPHP: invalid username %q", username)
	}
	if !phpVersionRegex.MatchString(version) {
		return fmt.Errorf("ensureUserCLIPHP: invalid version %q", version)
	}
	target := "/usr/bin/php" + version
	if fi, err := os.Stat(target); err != nil || fi.IsDir() {
		return fmt.Errorf("ensureUserCLIPHP: target %s missing", target)
	}
	home := filepath.Join("/home", username)
	if fi, err := os.Stat(home); err != nil || !fi.IsDir() {
		return nil // no home → nothing to wire
	}
	binDir := userCLIPHPBinDir(username)
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("ensureUserCLIPHP: mkdir %s: %w", binDir, err)
	}
	// Keep .jabali + bin root-owned so the tenant can't replace the wrapper.
	_ = os.Chown(filepath.Join(home, ".jabali"), 0, 0)
	_ = os.Chown(binDir, 0, 0)

	return replaceCLISymlink(filepath.Join(binDir, "php"), target)
}

// replaceCLISymlink points `link` at `target` idempotently: a symlink
// already pointing at target is a no-op (no churn on every reconciler
// tick); otherwise it is replaced atomically (symlink to a temp name then
// rename over the live one).
func replaceCLISymlink(link, target string) error {
	if cur, err := os.Readlink(link); err == nil && cur == target {
		return nil // already correct → no-op (no-change gate)
	}
	tmp := link + ".tmp"
	_ = os.Remove(tmp)
	if err := os.Symlink(target, tmp); err != nil {
		return fmt.Errorf("replaceCLISymlink: symlink: %w", err)
	}
	if err := os.Rename(tmp, link); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replaceCLISymlink: rename: %w", err)
	}
	return nil
}

// removeUserCLIPHP removes the per-user CLI php wrapper (pool teardown).
// Removes only the `php` symlink, leaving an empty .jabali/bin behind
// harmlessly; not removing the dirs avoids racing other .jabali users.
func removeUserCLIPHP(username string) error {
	if !phpPoolUsernameRegex.MatchString(username) {
		return fmt.Errorf("removeUserCLIPHP: invalid username %q", username)
	}
	link := filepath.Join(userCLIPHPBinDir(username), "php")
	if err := os.Remove(link); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removeUserCLIPHP: %w", err)
	}
	return nil
}
