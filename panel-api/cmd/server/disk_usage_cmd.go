// `jabali disk-usage` (Gitea #568). Operator-side mirror of the tenant
// /me/disk-usage surface (api/me_disk_usage.go):
//
//   - show <user>    — the last stored snapshot (cheap DB read), the same data
//     GET /me/disk-usage returns.
//   - refresh <user> — recompute live via api.ComputeDiskUsage (the SAME
//     aggregation POST /me/disk-usage/refresh runs: home quota
//     report + cached mailbox usage + db.size) and persist it
//     as the new snapshot.
//
// Uses the exported api.ComputeDiskUsage / api.DiskUsageResult so the numbers
// are identical to the GUI — one implementation, no drift.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"git.linux-hosting.co.il/shukivaknin/jabali2/internal/limits"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/api"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/repository"
)

func diskUsageSnapshotRepoFromDB() repository.DiskUsageSnapshotRepository {
	return repository.NewDiskUsageSnapshotRepository(sharedDB)
}

// diskUsageCfg builds the same DiskUsageConfig serve.go wires for the endpoint.
func diskUsageCfg() api.DiskUsageConfig {
	quotaMount := ""
	if m, err := limits.QuotaMountFor("/home"); err == nil {
		quotaMount = m
	}
	return api.DiskUsageConfig{
		Users:      userRepo(),
		Domains:    repository.NewDomainRepository(sharedDB),
		Mailboxes:  repository.NewMailboxRepository(sharedDB),
		Databases:  repository.NewDatabaseRepository(sharedDB),
		Agent:      sharedAgent,
		QuotaMount: quotaMount,
		Snapshots:  diskUsageSnapshotRepoFromDB(),
	}
}

func newDiskUsageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disk-usage",
		Short: "Inspect / refresh a tenant's disk-usage breakdown (files / email / databases)",
	}
	cmd.AddCommand(newDiskUsageShowCmd(), newDiskUsageRefreshCmd())
	return cmd
}

func printDiskUsage(res api.DiskUsageResult) {
	computed := "never (run refresh)"
	if res.ComputedAt != nil {
		computed = res.ComputedAt.UTC().Format("2006-01-02 15:04Z")
	}
	fmt.Printf("Total: %s   (home quota: %s)   computed %s\n",
		humanBytes(res.TotalBytes), fmtBytesLimit(res.QuotaBytes), computed)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "CATEGORY\tBYTES")
	fmt.Fprintf(w, "files\t%s\n", humanBytes(res.Files.Bytes))
	fmt.Fprintf(w, "email\t%s\n", humanBytes(res.Email.Bytes))
	fmt.Fprintf(w, "databases\t%s\n", humanBytes(res.Databases.Bytes))
	w.Flush()
}

func newDiskUsageShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "show <user-email|username|id>",
		Short:   "Show a tenant's last stored disk-usage snapshot",
		Args:    cobra.ExactArgs(1),
		PreRunE: requireDB,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()
			u, err := resolveUser(ctx, args[0])
			if err != nil {
				return err
			}
			var res api.DiskUsageResult
			res.Files.Items = []api.DiskUsageItem{}
			res.Email.Items = []api.DiskUsageItem{}
			res.Databases.Items = []api.DiskUsageItem{}
			if snap, serr := diskUsageSnapshotRepoFromDB().Get(ctx, u.ID); serr == nil && snap != nil {
				if uerr := json.Unmarshal([]byte(snap.Payload), &res); uerr == nil {
					ca := snap.ComputedAt
					res.ComputedAt = &ca
				}
			}
			if jsonOutput {
				return printJSON(res)
			}
			printDiskUsage(res)
			return nil
		},
	}
}

func newDiskUsageRefreshCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "refresh <user-email|username|id>",
		Short:   "Recompute a tenant's disk usage live and store the snapshot",
		Args:    cobra.ExactArgs(1),
		PreRunE: requireDBAndAgent,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 90*time.Second)
			defer cancel()
			u, err := resolveUser(ctx, args[0])
			if err != nil {
				return err
			}
			res, err := api.ComputeDiskUsage(ctx, diskUsageCfg(), u.ID)
			if err != nil {
				return fmt.Errorf("compute disk usage: %w", err)
			}
			now := time.Now()
			res.ComputedAt = &now
			if payload, merr := json.Marshal(res); merr == nil {
				_ = diskUsageSnapshotRepoFromDB().Upsert(ctx, u.ID, string(payload), now)
			}
			if jsonOutput {
				return printJSON(res)
			}
			printDiskUsage(res)
			return nil
		},
	}
}
