// Database operations CLI (Gitea #559): per-database backup/restore, admin
// maintenance + process management, and root-password rotation. Each dispatches
// the same agent verb the API does (databases.go / databases_admin_ops.go), with
// the same engine variants (mariadb/postgres). Config tuning (get/put) is
// deferred — it needs the dbtuning allowlist validation + DBAdmin tuning repo and
// is tracked for a follow-up. Mutations are CLI-audited.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/spf13/cobra"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/ids"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/models"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/repository"
)

func dbEngineValidCLI(e string) bool { return e == "mariadb" || e == "postgres" }

func resolveDatabaseCLI(ctx context.Context, ref string) (*models.Database, error) {
	repo := dbRepoFromDB()
	if d, err := repo.FindByID(ctx, ref); err == nil && d != nil {
		return d, nil
	}
	rows, _, err := repo.List(ctx, repository.ListOptions{})
	if err != nil {
		return nil, err
	}
	for i := range rows {
		if rows[i].Name == ref {
			return &rows[i], nil
		}
	}
	return nil, fmt.Errorf("no database with id or name %q", ref)
}

func newDBBackupCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "backup <db-id|db-name>",
		Short:   "Create a backup of a database (returns the dump path)",
		Args:    cobra.ExactArgs(1),
		PreRunE: requireDBAndAgent,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
			defer cancel()
			d, err := resolveDatabaseCLI(ctx, args[0])
			if err != nil {
				return err
			}
			raw, err := sharedAgent.Call(ctx, "db.backup", map[string]any{"db_name": d.Name})
			if err != nil {
				return fmt.Errorf("agent db.backup: %w", err)
			}
			cliAuditOK(ctx, "database.backup", "database", d.ID, &d.UserID)
			os.Stdout.Write(raw)
			os.Stdout.Write([]byte{'\n'})
			return nil
		},
	}
}

func newDBRestoreCmd() *cobra.Command {
	var file string
	var force bool
	cmd := &cobra.Command{
		Use:     "restore <db-id|db-name>",
		Short:   "Restore a database from a .sql dump on the host",
		Args:    cobra.ExactArgs(1),
		PreRunE: requireDBAndAgent,
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return fmt.Errorf("--file (path to a .sql dump on the host) is required")
			}
			if !force {
				return fmt.Errorf("restore overwrites the database contents; re-run with --force")
			}
			if _, err := os.Stat(file); err != nil {
				return fmt.Errorf("dump file not readable: %w", err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
			defer cancel()
			d, err := resolveDatabaseCLI(ctx, args[0])
			if err != nil {
				return err
			}
			if _, err := sharedAgent.Call(ctx, "db.restore", map[string]any{"db_name": d.Name, "path": file}); err != nil {
				return fmt.Errorf("agent db.restore: %w", err)
			}
			cliAuditOK(ctx, "database.restore", "database", d.ID, &d.UserID)
			fmt.Fprintf(cmd.OutOrStdout(), "Restored %s from %s\n", d.Name, file)
			return nil
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "path to a .sql dump on the host (required)")
	cmd.Flags().BoolVar(&force, "force", false, "confirm the overwrite")
	return cmd
}

func newDBProcessesCmd() *cobra.Command {
	var engine string
	cmd := &cobra.Command{
		Use:     "processes",
		Short:   "List database processes/activity",
		Args:    cobra.NoArgs,
		PreRunE: requireDBAndAgent,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !dbEngineValidCLI(engine) {
				return fmt.Errorf("--engine must be mariadb|postgres")
			}
			verb := "db.processlist"
			if engine == "postgres" {
				verb = "db.postgres.activity"
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 20*time.Second)
			defer cancel()
			return csCall(ctx, verb, map[string]any{})
		},
	}
	cmd.Flags().StringVar(&engine, "engine", "mariadb", "mariadb|postgres")
	return cmd
}

func newDBKillCmd() *cobra.Command {
	var engine string
	var force bool
	cmd := &cobra.Command{
		Use:     "kill <process-id>",
		Short:   "Kill/terminate a database process",
		Args:    cobra.ExactArgs(1),
		PreRunE: requireDBAndAgent,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !dbEngineValidCLI(engine) {
				return fmt.Errorf("--engine must be mariadb|postgres")
			}
			if !force {
				return fmt.Errorf("killing a DB process can abort a transaction; re-run with --force")
			}
			verb := "db.kill"
			if engine == "postgres" {
				verb = "db.postgres.terminate"
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()
			err := csCall(ctx, verb, map[string]any{"id": args[0]})
			if err == nil {
				cliAuditOK(ctx, "database.process_kill", "database_process", args[0], nil)
			}
			return err
		},
	}
	cmd.Flags().StringVar(&engine, "engine", "mariadb", "mariadb|postgres")
	cmd.Flags().BoolVar(&force, "force", false, "confirm the kill")
	return cmd
}

func newDBMaintenanceCmd() *cobra.Command {
	var engine, scope string
	cmd := &cobra.Command{
		Use:     "maintenance",
		Short:   "Run optimize/analyze (mariadb) or vacuum/analyze (postgres)",
		Args:    cobra.NoArgs,
		PreRunE: requireDBAndAgent,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !dbEngineValidCLI(engine) {
				return fmt.Errorf("--engine must be mariadb|postgres")
			}
			if scope == "" {
				scope = "all"
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Minute)
			defer cancel()
			adminRepo := repository.NewDBAdminRepository(sharedDB)
			if running, _ := adminRepo.RunningJob(ctx, engine); running != nil {
				return fmt.Errorf("maintenance already running for %s (job %s)", engine, running.ID)
			}
			job := &models.DBAdminJob{ID: ulid.Make().String(), Engine: engine, Kind: "maintenance", Scope: scope, Status: "running", ActorUserID: "cli"}
			if err := adminRepo.CreateJob(ctx, job); err != nil {
				return fmt.Errorf("create job: %w", err)
			}
			verb := "db.maintenance"
			if engine == "postgres" {
				verb = "db.postgres.maintenance"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Running %s maintenance (scope=%s, job %s) — this may take several minutes…\n", engine, scope, job.ID)
			raw, aerr := sharedAgent.Call(ctx, verb, map[string]any{"scope": scope})
			status, summary := "ok", ""
			if aerr != nil {
				status, summary = "error", aerr.Error()
			} else {
				var r struct {
					Summary string `json:"summary"`
				}
				_ = json.Unmarshal(raw, &r)
				summary = r.Summary
			}
			_ = adminRepo.FinishJob(ctx, job.ID, status, summary)
			cliAuditOK(ctx, "database.maintenance", "db_admin_job", job.ID, nil)
			if aerr != nil {
				return fmt.Errorf("maintenance failed: %w", aerr)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Maintenance %s: %s\n", status, summary)
			return nil
		},
	}
	cmd.Flags().StringVar(&engine, "engine", "mariadb", "mariadb|postgres")
	cmd.Flags().StringVar(&scope, "scope", "all", "'all' or a database name")
	return cmd
}

func newDBMaintenanceStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "maintenance-status <job-id>",
		Short:   "Show a maintenance job's status",
		Args:    cobra.ExactArgs(1),
		PreRunE: requireDB,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()
			job, err := repository.NewDBAdminRepository(sharedDB).GetJob(ctx, args[0])
			if err != nil {
				return fmt.Errorf("job not found: %w", err)
			}
			return printJSON(job)
		},
	}
}

func newDBRootPasswordCmd() *cobra.Command {
	var engine string
	cmd := &cobra.Command{
		Use:     "root-password",
		Short:   "Rotate the database root/superuser password (revealed once)",
		Args:    cobra.NoArgs,
		PreRunE: requireDBAndAgent,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !dbEngineValidCLI(engine) {
				return fmt.Errorf("--engine must be mariadb|postgres")
			}
			pw := ids.NewSecret()
			verb := "db.root.set_password"
			if engine == "postgres" {
				verb = "db.postgres.superuser.set_password"
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			if _, err := sharedAgent.Call(ctx, verb, map[string]any{"new_password": pw}); err != nil {
				cliAuditErr(ctx, "database.root_password_rotate", "database_engine", engine, nil)
				return fmt.Errorf("agent %s: %w", verb, err)
			}
			cliAuditOK(ctx, "database.root_password_rotate", "database_engine", engine, nil)
			if jsonOutput {
				return printJSON(map[string]any{"engine": engine, "password": pw})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Rotated %s root/superuser password.\nNew password: %s\n(Shown once — store it now.)\n", engine, pw)
			return nil
		},
	}
	cmd.Flags().StringVar(&engine, "engine", "mariadb", "mariadb|postgres")
	return cmd
}

func registerDBOpsCmds(parent *cobra.Command) {
	parent.AddCommand(
		newDBBackupCmd(), newDBRestoreCmd(), newDBProcessesCmd(), newDBKillCmd(),
		newDBMaintenanceCmd(), newDBMaintenanceStatusCmd(), newDBRootPasswordCmd(),
	)
}
