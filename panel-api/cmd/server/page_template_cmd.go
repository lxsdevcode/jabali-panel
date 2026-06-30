// Page template management CLI (Gitea #564). Same repo the /admin/settings/
// page-templates routes use: list/get/upsert + reset to the built-in default.
// The reconciler syncs rendered error pages to disk on its next pass.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/repository"
)

func newPageTemplateCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "page-template", Short: "Manage error/index page templates"}
	cmd.AddCommand(newPageTemplateListCmd(), newPageTemplateGetCmd(), newPageTemplateSetCmd(), newPageTemplateResetCmd())
	return cmd
}

func newPageTemplateListCmd() *cobra.Command {
	return &cobra.Command{
		Use: "list", Short: "List page templates", Args: cobra.NoArgs, PreRunE: requireDB,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()
			rows, err := repository.NewPageTemplateRepository(sharedDB).List(ctx)
			if err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(rows)
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			fmt.Fprintln(tw, "KEY\tBYTES\tUPDATED")
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%d\t%s\n", r.Key, len(r.Content), r.UpdatedAt.Format("2006-01-02 15:04"))
			}
			return tw.Flush()
		},
	}
}

func newPageTemplateGetCmd() *cobra.Command {
	return &cobra.Command{
		Use: "get <key>", Short: "Print a page template's content", Args: cobra.ExactArgs(1), PreRunE: requireDB,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()
			row, err := repository.NewPageTemplateRepository(sharedDB).Get(ctx, args[0])
			if err != nil {
				return fmt.Errorf("template %q not found: %w", args[0], err)
			}
			if jsonOutput {
				return printJSON(row)
			}
			fmt.Fprint(cmd.OutOrStdout(), row.Content)
			return nil
		},
	}
}

func newPageTemplateSetCmd() *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use: "set <key>", Short: "Set a page template's content from a file", Args: cobra.ExactArgs(1), PreRunE: requireDB,
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return fmt.Errorf("--file is required")
			}
			content, err := os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("read template file: %w", err)
			}
			if len(content) > 128*1024 {
				return fmt.Errorf("template too large (max 128 KiB)")
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()
			if err := repository.NewPageTemplateRepository(sharedDB).Upsert(ctx, args[0], string(content)); err != nil {
				return err
			}
			cliAuditOK(ctx, "page_template.set", "page_template", args[0], nil)
			fmt.Fprintf(cmd.OutOrStdout(), "Set template %s (%d bytes). Reconciler syncs error pages on the next pass.\n", args[0], len(content))
			return nil
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "path to the template content (required)")
	return cmd
}

func newPageTemplateResetCmd() *cobra.Command {
	return &cobra.Command{
		Use: "reset <key>", Short: "Reset a page template to the built-in default", Args: cobra.ExactArgs(1), PreRunE: requireDB,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()
			body := repository.DefaultPageTemplateBody(args[0])
			if body == "" {
				return fmt.Errorf("no built-in default for key %q", args[0])
			}
			if err := repository.NewPageTemplateRepository(sharedDB).Upsert(ctx, args[0], body); err != nil {
				return err
			}
			cliAuditOK(ctx, "page_template.reset", "page_template", args[0], nil)
			fmt.Fprintf(cmd.OutOrStdout(), "Reset %s to default.\n", args[0])
			return nil
		},
	}
}

// ---- #565 active tasks (read-only aggregate, same sources as /admin/active-tasks) ----

func newActiveTasksCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "active-tasks",
		Short:   "Show running/queued tasks (updates, backups, malware scans)",
		Args:    cobra.NoArgs,
		PreRunE: requireDBAndAgent,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			updates, _ := repository.NewUpdateHistoryRepository(sharedDB).ListRunning(ctx)
			bjRepo := repository.NewBackupJobRepository(sharedDB)
			since := time.Now().Add(-24 * time.Hour)
			backups := []any{}
			for _, st := range []string{"running", "queued"} {
				if jobs, err := bjRepo.ListByStatusSince(ctx, st, since, 20); err == nil {
					for i := range jobs {
						backups = append(backups, jobs[i])
					}
				}
			}
			malwareScanning := false
			if raw, err := sharedAgent.Call(ctx, "security.malware.active_scans", nil); err == nil {
				var v struct {
					Active int `json:"active"`
				}
				if json.Unmarshal(raw, &v) == nil && v.Active > 0 {
					malwareScanning = true
				}
			}
			count := len(updates) + len(backups)
			if malwareScanning {
				count++
			}
			if jsonOutput {
				return printJSON(map[string]any{
					"updates": updates, "backups": backups,
					"malware_scanning": malwareScanning, "count": count,
				})
			}
			if count == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No active tasks.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d active task(s): %d update(s), %d backup job(s)%s\n",
				count, len(updates), len(backups), map[bool]string{true: ", malware scan running", false: ""}[malwareScanning])
			return printJSON(map[string]any{"updates": updates, "backups": backups, "malware_scanning": malwareScanning})
		},
	}
}
