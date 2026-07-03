package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// migrate_refresh_cmd.go — GH #646. `jabali migrate refresh` orchestrates the
// destructive re-migration: backup FIRST (abort on failure), then force-mirror
// files (preserving jabali-managed files), drop+reimport the DB, and reconcile.
// Gated behind --force (refuses on a live host without it, like account-restore).

func newMigrateRefreshCmd() *cobra.Command {
	var (
		docroot, dbName, osUser, domain string
		srcDocroot, srcSQL              string
		oldURL, newURL                  string
		force                           bool
	)
	cmd := &cobra.Command{
		Use:     "refresh",
		Short:   "Force re-pull (refresh) an already-migrated account from a staged source",
		Long: `Overwrites a live migrated account: dest files are mirrored from the
staged source (--delete, preserving wp-config.php + jabali drop-ins) and the
dest DB is dropped + reimported. A hardlink snapshot + DB dump are taken FIRST;
the refresh aborts if that backup fails. Requires --force.`,
		PreRunE: requireDBAndAgent,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !force {
				return fmt.Errorf("refresh overwrites a live account — pass --force to proceed (a pre-overwrite backup is taken automatically)")
			}
			if docroot == "" || dbName == "" || osUser == "" || srcDocroot == "" || srcSQL == "" {
				return fmt.Errorf("--docroot, --db, --os-user, --source-docroot, --source-sql are required")
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Minute)
			defer cancel()
			stamp := time.Now().UTC().Format("20060102-150405")

			// Wave B: mandatory backup FIRST.
			bres, err := sharedAgent.Call(ctx, "migration.refresh_backup", map[string]any{
				"docroot": docroot, "os_user": osUser, "db_name": dbName, "stamp": stamp,
			})
			if err != nil {
				return fmt.Errorf("pre-overwrite backup FAILED — aborting, dest untouched: %w", err)
			}
			var b struct {
				Snapshot string `json:"snapshot"`
				DBDump   string `json:"db_dump"`
			}
			_ = json.Unmarshal(bres, &b)
			fmt.Printf("backup ok — snapshot=%s db_dump=%s\n", b.Snapshot, b.DBDump)
			fmt.Printf("(rollback: rsync the snapshot back + `mysql %s < %s`)\n", dbName, b.DBDump)

			// Wave C: force writers.
			if _, err := sharedAgent.Call(ctx, "migration.refresh_files", map[string]any{
				"source": srcDocroot, "docroot": docroot, "os_user": osUser,
			}); err != nil {
				return fmt.Errorf("file mirror failed (backup preserved at %s): %w", b.Snapshot, err)
			}
			fmt.Println("files mirrored (jabali-managed files preserved)")
			if _, err := sharedAgent.Call(ctx, "migration.refresh_db", map[string]any{
				"db_name": dbName, "sql_path": srcSQL,
			}); err != nil {
				return fmt.Errorf("DB reimport failed (backup preserved at %s / %s): %w", b.Snapshot, b.DBDump, err)
			}
			fmt.Println("DB dropped + reimported")

			// Wave D: reconcile.
			if _, err := sharedAgent.Call(ctx, "migration.refresh_reconcile", map[string]any{
				"os_user": osUser, "install_path": docroot, "domain": domain,
				"old_url": oldURL, "new_url": newURL,
			}); err != nil {
				fmt.Printf("(reconcile warning: %v — files + DB are refreshed)\n", err)
			}
			if domain != "" {
				_, _ = sharedAgent.Call(ctx, "nginx.cache.purge", map[string]any{"domain": domain})
			}
			fmt.Printf("refresh complete. Backup retained at %s (+ %s) — remove once verified.\n", b.Snapshot, b.DBDump)
			return nil
		},
	}
	cmd.Flags().StringVar(&docroot, "docroot", "", "Dest docroot (overwritten)")
	cmd.Flags().StringVar(&dbName, "db", "", "Dest DB name (identity unchanged)")
	cmd.Flags().StringVar(&osUser, "os-user", "", "Dest Linux user")
	cmd.Flags().StringVar(&domain, "domain", "", "Dest domain (for cache purge)")
	cmd.Flags().StringVar(&srcDocroot, "source-docroot", "", "Staged source docroot")
	cmd.Flags().StringVar(&srcSQL, "source-sql", "", "Staged source SQL dump")
	cmd.Flags().StringVar(&oldURL, "old-url", "", "Old site URL (search-replace)")
	cmd.Flags().StringVar(&newURL, "new-url", "", "New site URL (search-replace)")
	cmd.Flags().BoolVar(&force, "force", false, "Required — refresh overwrites a live account")
	return cmd
}
