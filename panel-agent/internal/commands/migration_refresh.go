package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

)

// migration_refresh.go — GH #646. Re-migration/refresh writers. The refresh is
// DESTRUCTIVE (overwrites a live migrated account), so Wave B (backup) is a hard
// precondition enforced by the runner: no refresh writer runs until
// migration.refresh_backup has produced a durable dest snapshot + DB dump.

var (
	refreshDBNameRE  = regexp.MustCompile(`^[A-Za-z0-9_]{1,64}$`)
	refreshStampRE   = regexp.MustCompile(`^[0-9]{8}-[0-9]{6}$`)
	refreshOSUserRE  = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)
)

// jabaliManagedRefreshExcludes are files the file mirror must NEVER overwrite —
// they carry the DEST's jabali integration (DB creds, Redis/PHP drop-ins). The
// source's copies would break the dest (invariant 2, GH #646).
var jabaliManagedRefreshExcludes = []string{
	"wp-config.php",
	"wp-content/object-cache.php",
	"wp-content/advanced-cache.php",
	".user.ini",
	"wp-content/mu-plugins/jabali-sso-*.php",
}

func refreshValidateCommon(docroot, osUser string) (string, error) {
	if !strings.HasPrefix(docroot, "/home/") || strings.Contains(docroot, "..") {
		return "", fmt.Errorf("docroot must be under /home with no ..")
	}
	if !refreshOSUserRE.MatchString(osUser) {
		return "", fmt.Errorf("invalid os_user")
	}
	return filepath.Clean(docroot), nil
}

type refreshBackupParams struct {
	Docroot string `json:"docroot"`
	OSUser  string `json:"os_user"`
	DBName  string `json:"db_name"`
	Stamp   string `json:"stamp"` // YYYYMMDD-HHMMSS
}

// migration.refresh_backup (GH #646 Wave B) — MANDATORY pre-overwrite backup:
// a hardlink snapshot of the dest docroot (near-instant, cheap) + a mysqldump of
// the dest DB. Non-destructive. A failure here aborts the refresh (runner) with
// the dest untouched.
func migrationRefreshBackupHandler(ctx context.Context, raw json.RawMessage) (any, error) {
	var p refreshBackupParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, csInvalidArg("bad params")
	}
	docroot, err := refreshValidateCommon(p.Docroot, p.OSUser)
	if err != nil {
		return nil, csInvalidArg(err.Error())
	}
	if !refreshDBNameRE.MatchString(p.DBName) {
		return nil, csInvalidArg("invalid db_name")
	}
	if !refreshStampRE.MatchString(p.Stamp) {
		return nil, csInvalidArg("stamp must be YYYYMMDD-HHMMSS")
	}
	if fi, sErr := os.Stat(docroot); sErr != nil || !fi.IsDir() {
		return nil, csInvalidArg("docroot does not exist")
	}

	// 1. Hardlink snapshot of the docroot (cp -al: instant, shares inodes).
	snapshot := docroot + ".pre-refresh-" + p.Stamp
	if _, sErr := os.Stat(snapshot); sErr == nil {
		return nil, csInvalidArg("snapshot already exists: " + snapshot)
	}
	if out, cErr := exec.CommandContext(ctx, "cp", "-al", docroot, snapshot).CombinedOutput(); cErr != nil {
		return nil, csInternal("docroot snapshot", fmt.Errorf("%v: %s", cErr, string(out)))
	}

	// 2. mysqldump the dest DB alongside the snapshot.
	dumpDir := filepath.Dir(docroot)
	dumpPath := filepath.Join(dumpDir, "pre-refresh-"+p.Stamp+".sql")
	f, fErr := os.Create(dumpPath)
	if fErr != nil {
		return nil, csInternal("create dump", fErr)
	}
	defer f.Close()
	dump := exec.CommandContext(ctx, "mysqldump", "--single-transaction", "--quick", "--lock-tables=0", "--", p.DBName)
	dump.Stdout = f
	if dErr := dump.Run(); dErr != nil {
		_ = os.Remove(dumpPath)
		// Roll back the snapshot too — a partial backup is not a backup.
		_ = os.RemoveAll(snapshot)
		return nil, csInternal("mysqldump", dErr)
	}
	return map[string]any{"ok": true, "snapshot": snapshot, "db_dump": dumpPath}, nil
}

func init() {
	Default.Register("migration.refresh_backup", migrationRefreshBackupHandler)
}
