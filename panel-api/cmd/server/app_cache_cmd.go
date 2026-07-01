// app_cache_cmd.go — GH #556 CLI parity for the application cache toggle.
// Reuses the exact backend core as the web PUT /applications/:id/cache path
// (api.SetApplicationCache) so validation, ACL provisioning, and the agent
// wordpress.cache_set call stay authoritative. Operator context => admin.
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/api"
)

func newAppCacheCmd() *cobra.Command {
	var enable, disable bool
	cmd := &cobra.Command{
		Use:     "cache <install-id>",
		Short:   "Enable/disable object + page cache for a WordPress app install",
		Args:    cobra.ExactArgs(1),
		PreRunE: requireDBAndAgent,
		RunE: func(cmd *cobra.Command, args []string) error {
			if enable == disable {
				return fmt.Errorf("specify exactly one of --enable or --disable")
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
			defer cancel()

			cfg, err := buildAppDeps()
			if err != nil {
				return fmt.Errorf("wire app deps: %w", err)
			}
			installID := args[0]

			// Operator CLI acts as admin (actorUserID is unused in the admin
			// path). Reconciler is nil here, so the per-domain page-cache vhost
			// re-renders on the next reconcile tick rather than immediately.
			if err := api.SetApplicationCache(ctx, cfg, installID, enable, true, ""); err != nil {
				cliAuditErr(ctx, "app.cache", "app_install", installID, nil)
				return fmt.Errorf("set cache: %w", err)
			}
			cliAuditOK(ctx, "app.cache", "app_install", installID, nil)

			state := "enabled"
			if disable {
				state = "disabled"
			}
			fmt.Printf("cache %s for install %s; the page-cache vhost converges within ~60s\n", state, installID)
			return nil
		},
	}
	cmd.Flags().BoolVar(&enable, "enable", false, "enable object + page cache")
	cmd.Flags().BoolVar(&disable, "disable", false, "disable object + page cache")
	return cmd
}
