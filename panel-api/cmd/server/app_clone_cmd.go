// app_clone_cmd.go — GH #556 CLI parity for the WordPress clone flow.
// Reuses the exact backend core as the web POST /applications/:id/clone path
// (api.CloneApplication) so DB/user/grant provisioning, agent calls, rollback,
// and the file/DB copy stay authoritative.
//
// Ownership: the web handler ties the clone to the caller and 403s if they
// don't own the dest domain — so an operator passing isAdmin=true would always
// 403. The CLI instead resolves the dest-domain owner and clones AS them
// (isAdmin=false, actorUserID=owner), the same code path as a tenant cloning
// their own site. async=false blocks until the agent copy finishes, since the
// CLI process must stay alive for the kick to complete.
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/api"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/repository"
)

func newAppCloneCmd() *cobra.Command {
	var toDomain string
	cmd := &cobra.Command{
		Use:     "clone <src-install-id>",
		Short:   "Clone a WordPress app install onto another domain (files + DB)",
		Args:    cobra.ExactArgs(1),
		PreRunE: requireDBAndAgent,
		RunE: func(cmd *cobra.Command, args []string) error {
			if toDomain == "" {
				return fmt.Errorf("--to-domain is required (dest domain ID or name)")
			}
			// The agent kick runs up to 5 min; give it headroom.
			ctx, cancel := context.WithTimeout(cmd.Context(), 8*time.Minute)
			defer cancel()

			cfg, err := buildAppDeps()
			if err != nil {
				return fmt.Errorf("wire app deps: %w", err)
			}
			srcInstallID := args[0]

			// Resolve the dest domain (ID or name) and its owner.
			destDomain, err := resolveDomainSpec(ctx, domainRepoFromDB(), toDomain)
			if err != nil {
				return fmt.Errorf("resolve dest domain: %w", err)
			}

			// Clone as the dest-domain owner (isAdmin=false), blocking until the
			// agent copy completes (async=false).
			resp, err := api.CloneApplication(ctx, cfg, srcInstallID, destDomain.ID, false, destDomain.UserID, false)
			if err != nil {
				cliAuditErr(ctx, "app.clone", "app_install", srcInstallID, &destDomain.UserID)
				return fmt.Errorf("clone: %w", err)
			}

			// The blocking kick (createCloneAndKickAgent) is void — it flips the
			// row to "failed" on error rather than returning one, so
			// CloneApplication returns nil even when the copy failed. Re-read the
			// row and surface a non-zero exit on failure (GH silent-exit-0 scar).
			instRepo := repository.NewApplicationInstallRepository(sharedDB)
			status := resp.Status
			var lastErr string
			if inst, fErr := instRepo.FindByID(ctx, resp.ID); fErr == nil && inst != nil {
				status = inst.Status
				lastErr = inst.LastError
			}
			if status == "failed" {
				cliAuditErr(ctx, "app.clone", "app_install", resp.ID, &destDomain.UserID)
				if lastErr != "" {
					return fmt.Errorf("clone %s failed: %s", resp.ID, lastErr)
				}
				return fmt.Errorf("clone %s failed", resp.ID)
			}
			cliAuditOK(ctx, "app.clone", "app_install", resp.ID, &destDomain.UserID)
			fmt.Printf("cloned install %s onto %s (domain %s); status=%s\n", resp.ID, destDomain.Name, destDomain.ID, status)
			return nil
		},
	}
	cmd.Flags().StringVar(&toDomain, "to-domain", "", "destination domain (ID or name)")
	return cmd
}
