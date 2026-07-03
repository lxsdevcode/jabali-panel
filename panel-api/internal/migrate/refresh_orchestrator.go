package migrate

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// refresh_orchestrator.go — GH #646. Shared re-migration orchestration used by
// both the `jabali migrate refresh` CLI and the panel admin endpoint. Runs the
// DESTRUCTIVE refresh in strict order: backup FIRST (abort on failure), then the
// file mirror + DB reimport, then reconcile + cache purge. Backup-before-overwrite
// is enforced here structurally — the writers only run after backup succeeds.

// RefreshAgent is the minimal agent surface the orchestration needs.
type RefreshAgent interface {
	Call(ctx context.Context, command string, params any) (json.RawMessage, error)
}

// RefreshInput is the dest + staged-source description.
type RefreshInput struct {
	Docroot    string // dest docroot (overwritten)
	DBName     string // dest DB (identity unchanged)
	OSUser     string // dest Linux user
	Domain     string // dest domain (cache purge)
	SrcDocroot string // staged source docroot
	SrcSQL     string // staged source SQL dump
	OldURL     string // search-replace (optional)
	NewURL     string
}

// RefreshResult is the outcome, including the retained backup for rollback.
type RefreshResult struct {
	Snapshot string   `json:"snapshot"`
	DBDump   string   `json:"db_dump"`
	Warnings []string `json:"warnings,omitempty"`
}

// RunRefresh executes the full refresh. Requires the caller to have already
// gated on --force / an explicit confirm.
func RunRefresh(ctx context.Context, agent RefreshAgent, in RefreshInput, nowUTC time.Time) (*RefreshResult, error) {
	if in.Docroot == "" || in.DBName == "" || in.OSUser == "" || in.SrcDocroot == "" || in.SrcSQL == "" {
		return nil, fmt.Errorf("docroot, db, os_user, source_docroot, source_sql are required")
	}
	stamp := nowUTC.Format("20060102-150405")

	// Wave B: mandatory backup FIRST — abort with the dest untouched on failure.
	bres, err := agent.Call(ctx, "migration.refresh_backup", map[string]any{
		"docroot": in.Docroot, "os_user": in.OSUser, "db_name": in.DBName, "stamp": stamp,
	})
	if err != nil {
		return nil, fmt.Errorf("pre-overwrite backup FAILED — aborted, dest untouched: %w", err)
	}
	res := &RefreshResult{}
	_ = json.Unmarshal(bres, res)

	// Wave C: force writers.
	if _, err := agent.Call(ctx, "migration.refresh_files", map[string]any{
		"source": in.SrcDocroot, "docroot": in.Docroot, "os_user": in.OSUser,
	}); err != nil {
		return res, fmt.Errorf("file mirror failed (backup preserved at %s): %w", res.Snapshot, err)
	}
	if _, err := agent.Call(ctx, "migration.refresh_db", map[string]any{
		"db_name": in.DBName, "sql_path": in.SrcSQL,
	}); err != nil {
		return res, fmt.Errorf("DB reimport failed (backup preserved at %s / %s): %w", res.Snapshot, res.DBDump, err)
	}

	// Wave D: reconcile + purge (non-fatal).
	if _, err := agent.Call(ctx, "migration.refresh_reconcile", map[string]any{
		"os_user": in.OSUser, "install_path": in.Docroot, "domain": in.Domain,
		"old_url": in.OldURL, "new_url": in.NewURL,
	}); err != nil {
		res.Warnings = append(res.Warnings, "reconcile: "+err.Error())
	}
	if in.Domain != "" {
		_, _ = agent.Call(ctx, "nginx.cache.purge", map[string]any{"domain": in.Domain})
	}
	return res, nil
}
