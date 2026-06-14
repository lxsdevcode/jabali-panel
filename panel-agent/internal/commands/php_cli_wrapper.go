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
//
// SECURITY (ADR-0126): the agent runs as root and writes under /home/<user>.
// On a jabali host that home is root-owned 0751, but we do NOT rely on that
// implicitly — every path component we touch is Lstat'd and refused if it is
// a symlink or not root-owned, so a tenant who *did* control their home could
// not use a planted symlink to redirect root's mkdir/chown/symlink/rename
// into privileged space (symlink TOCTOU). Chowns use Lchown (never follow a
// link) and the wrapper is replaced only when the existing entry is absent or
// already a symlink — never over a regular file/dir. The symlink target is
// always a validated /usr/bin/php<version> that exists; never tenant-input.
package commands

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// userCLIPHPBinDir returns the per-user wrapper bin dir for a username.
func userCLIPHPBinDir(username string) string {
	return filepath.Join("/home", username, ".jabali", "bin")
}

// isRootOwned reports whether the stat info belongs to uid 0.
func isRootOwned(fi os.FileInfo) bool {
	st, ok := fi.Sys().(*syscall.Stat_t)
	return ok && st.Uid == 0
}

// ensureRootDir makes `dir` a root-owned, symlink-free directory. If it
// already exists it must be a non-symlink dir owned by root, else we refuse
// (a tenant-planted symlink or tenant-owned dir is never followed/written
// into). Creates with Mkdir (not MkdirAll — the parent is verified by the
// caller) + Lchown to root.
func ensureRootDir(dir string) error {
	fi, err := os.Lstat(dir)
	if err == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refuse: %s is a symlink", dir)
		}
		if !fi.IsDir() {
			return fmt.Errorf("refuse: %s is not a directory", dir)
		}
		if !isRootOwned(fi) {
			return fmt.Errorf("refuse: %s is not root-owned", dir)
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	if err := os.Mkdir(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	return os.Lchown(dir, 0, 0)
}

// ensureUserCLIPHP writes/refreshes /home/<user>/.jabali/bin/php as a symlink
// to /usr/bin/php<version>. Idempotent and symlink-TOCTOU-safe (see file
// header). A user without a root-owned /home/<user> directory (no home, or a
// tenant-writable/symlinked home) is refused rather than written into.
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
	hfi, err := os.Lstat(home)
	if err != nil {
		return nil // no home (system/service account) → nothing to wire
	}
	if hfi.Mode()&os.ModeSymlink != 0 || !hfi.IsDir() || !isRootOwned(hfi) {
		// A symlinked, non-dir, or tenant-owned home is unsafe to write
		// under as root. Skip rather than risk a redirected write.
		return fmt.Errorf("ensureUserCLIPHP: %s is not a root-owned directory", home)
	}

	jabaliDir := filepath.Join(home, ".jabali")
	if err := ensureRootDir(jabaliDir); err != nil {
		return err
	}
	binDir := filepath.Join(jabaliDir, "bin")
	if err := ensureRootDir(binDir); err != nil {
		return err
	}
	return replaceCLISymlink(filepath.Join(binDir, "php"), target)
}

// replaceCLISymlink points `link` at `target` idempotently and safely: an
// existing entry that is NOT a symlink is refused (never overwrite a regular
// file/dir a tenant may have planted); a symlink already pointing at target
// is a no-op; otherwise it is replaced atomically (symlink to a temp name
// then rename over the live one).
func replaceCLISymlink(link, target string) error {
	if fi, err := os.Lstat(link); err == nil {
		if fi.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("refuse: %s exists and is not a symlink", link)
		}
		if cur, rerr := os.Readlink(link); rerr == nil && cur == target {
			return nil // already correct → no-op (no-change gate)
		}
	} else if !os.IsNotExist(err) {
		return err
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
// Removes only the `php` symlink (and only if it IS a symlink), leaving an
// empty .jabali/bin behind harmlessly.
func removeUserCLIPHP(username string) error {
	if !phpPoolUsernameRegex.MatchString(username) {
		return fmt.Errorf("removeUserCLIPHP: invalid username %q", username)
	}
	link := filepath.Join(userCLIPHPBinDir(username), "php")
	fi, err := os.Lstat(link)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("removeUserCLIPHP: %w", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("refuse: %s is not a symlink", link)
	}
	if err := os.Remove(link); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removeUserCLIPHP: %w", err)
	}
	return nil
}

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
