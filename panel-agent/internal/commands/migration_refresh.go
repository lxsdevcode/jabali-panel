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
	refreshDBNameRE = regexp.MustCompile(`^[A-Za-z0-9_]{1,64}$`)
	refreshStampRE  = regexp.MustCompile(`^[0-9]{8}-[0-9]{6}$`)
	refreshOSUserRE = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)
	refreshTableRE  = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
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
	Stamp   string `json:"stamp"`
}

// migration.refresh_backup (Wave B) — MANDATORY pre-overwrite backup: a hardlink
// snapshot of the dest docroot + a mysqldump of the dest DB. Non-destructive; a
// failure aborts the refresh with the dest untouched.
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
	snapshot := docroot + ".pre-refresh-" + p.Stamp
	if _, sErr := os.Stat(snapshot); sErr == nil {
		return nil, csInvalidArg("snapshot already exists: " + snapshot)
	}
	if out, cErr := exec.CommandContext(ctx, "cp", "-al", docroot, snapshot).CombinedOutput(); cErr != nil {
		return nil, csInternal("docroot snapshot", fmt.Errorf("%v: %s", cErr, string(out)))
	}
	dumpPath := filepath.Join(filepath.Dir(docroot), "pre-refresh-"+p.Stamp+".sql")
	f, fErr := os.Create(dumpPath)
	if fErr != nil {
		return nil, csInternal("create dump", fErr)
	}
	defer f.Close()
	dump := exec.CommandContext(ctx, "mysqldump", "--single-transaction", "--quick", "--lock-tables=0", "--", p.DBName)
	dump.Stdout = f
	if dErr := dump.Run(); dErr != nil {
		_ = os.Remove(dumpPath)
		_ = os.RemoveAll(snapshot)
		return nil, csInternal("mysqldump", dErr)
	}
	return map[string]any{"ok": true, "snapshot": snapshot, "db_dump": dumpPath}, nil
}

type refreshFilesParams struct {
	Source  string `json:"source"`
	Docroot string `json:"docroot"`
	OSUser  string `json:"os_user"`
}

// migration.refresh_files (Wave C) — DESTRUCTIVE mirror of the staged source
// onto the dest docroot with --delete, EXCLUDING jabali-managed files so the
// dest DB creds + Redis/PHP integration survive. Re-applies <user>:www-data.
func migrationRefreshFilesHandler(ctx context.Context, raw json.RawMessage) (any, error) {
	var p refreshFilesParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, csInvalidArg("bad params")
	}
	docroot, err := refreshValidateCommon(p.Docroot, p.OSUser)
	if err != nil {
		return nil, csInvalidArg(err.Error())
	}
	src := filepath.Clean(p.Source)
	if !strings.HasPrefix(src, "/") || strings.Contains(src, "..") {
		return nil, csInvalidArg("source must be an absolute path with no ..")
	}
	if fi, sErr := os.Stat(src); sErr != nil || !fi.IsDir() {
		return nil, csInvalidArg("source dir does not exist")
	}
	if fi, dErr := os.Stat(docroot); dErr != nil || !fi.IsDir() {
		return nil, csInvalidArg("dest docroot does not exist")
	}
	args := []string{"-rlptD", "--delete"}
	for _, ex := range jabaliManagedRefreshExcludes {
		args = append(args, "--exclude="+ex)
	}
	args = append(args, src+"/", docroot+"/")
	if out, rErr := exec.CommandContext(ctx, "rsync", args...).CombinedOutput(); rErr != nil {
		return nil, csInternal("rsync mirror", fmt.Errorf("%v: %s", rErr, string(out)))
	}
	if out, cErr := exec.CommandContext(ctx, "chown", "-R", p.OSUser+":www-data", docroot).CombinedOutput(); cErr != nil {
		return nil, csInternal("chown docroot", fmt.Errorf("%v: %s", cErr, string(out)))
	}
	return map[string]any{"ok": true}, nil
}

type refreshDBParams struct {
	DBName  string `json:"db_name"`
	SQLPath string `json:"sql_path"`
}

// migration.refresh_db (Wave C) — DESTRUCTIVE: drop every table in the dest DB
// then reimport the staged source dump INTO THE SAME dest DB (same name+prefix)
// so the preserved dest wp-config stays valid.
func migrationRefreshDBHandler(ctx context.Context, raw json.RawMessage) (any, error) {
	var p refreshDBParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, csInvalidArg("bad params")
	}
	if !refreshDBNameRE.MatchString(p.DBName) {
		return nil, csInvalidArg("invalid db_name")
	}
	sql := filepath.Clean(p.SQLPath)
	if !strings.HasPrefix(sql, "/") || strings.Contains(sql, "..") {
		return nil, csInvalidArg("sql_path must be absolute with no ..")
	}
	if fi, sErr := os.Stat(sql); sErr != nil || fi.IsDir() {
		return nil, csInvalidArg("sql dump not found")
	}
	tblOut, tErr := exec.CommandContext(ctx, "mysql", "-N", "-e", "SHOW TABLES", "--", p.DBName).Output()
	if tErr != nil {
		return nil, csInternal("list dest tables", tErr)
	}
	var drop strings.Builder
	drop.WriteString("SET FOREIGN_KEY_CHECKS=0;\n")
	for _, t := range strings.Fields(string(tblOut)) {
		if refreshTableRE.MatchString(t) {
			drop.WriteString("DROP TABLE IF EXISTS `" + t + "`;\n")
		}
	}
	drop.WriteString("SET FOREIGN_KEY_CHECKS=1;\n")
	dropCmd := exec.CommandContext(ctx, "mysql", "--", p.DBName)
	dropCmd.Stdin = strings.NewReader(drop.String())
	if out, dErr := dropCmd.CombinedOutput(); dErr != nil {
		return nil, csInternal("drop dest tables", fmt.Errorf("%v: %s", dErr, string(out)))
	}
	f, fErr := os.Open(sql)
	if fErr != nil {
		return nil, csInternal("open dump", fErr)
	}
	defer f.Close()
	imp := exec.CommandContext(ctx, "mysql", "--", p.DBName)
	imp.Stdin = f
	if out, iErr := imp.CombinedOutput(); iErr != nil {
		return nil, csInternal("reimport dump", fmt.Errorf("%v: %s", iErr, string(out)))
	}
	return map[string]any{"ok": true}, nil
}

func init() {
	Default.Register("migration.refresh_backup", migrationRefreshBackupHandler)
	Default.Register("migration.refresh_files", migrationRefreshFilesHandler)
	Default.Register("migration.refresh_db", migrationRefreshDBHandler)
}
