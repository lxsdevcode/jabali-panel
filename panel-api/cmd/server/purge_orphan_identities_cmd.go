package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"git.jabali-panel.com/shukivaknin/jabali2/internal/kratosclient"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/repository"
)

// newAdminPurgeOrphanIdentitiesCmd deletes Kratos identities that no panel user
// row references (`users.kratos_identity_id`). These orphans are left behind by
// any user-delete that removed the DB row but not the identity — which was the
// case before the delete handler started calling DeleteIdentity (GH#132). An
// orphaned identity keeps its username/email claimed, so recreating the same
// user collides ("username_taken" / "user still there"). This command frees
// those names.
//
// Dry-run by default; --apply to delete. --username scopes the purge to a
// single orphan (matched on the identity's username trait, else email) so an
// operator can clear exactly one stuck name without touching anything else.
// An identity that DOES have a panel user row is never touched.
func newAdminPurgeOrphanIdentitiesCmd() *cobra.Command {
	var apply bool
	var onlyUsername string
	cmd := &cobra.Command{
		Use:   "purge-orphan-identities",
		Short: "Delete Kratos identities with no matching panel user (frees stuck usernames)",
		Long: `Walks every Kratos identity and deletes the ones with no corresponding
panel user row (orphans left by an older user-delete that didn't remove the
identity). Frees the username/email so the user can be recreated. Dry-run by
default; --apply to delete. --username <name> limits the purge to one orphan.`,
		PreRunE: requireDB,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
			defer cancel()

			kcfg := sharedCfg.Auth.Kratos
			if kcfg.AdminURL == "" {
				return fmt.Errorf("auth.kratos.admin_url not configured — cannot purge without admin API access")
			}
			kc := kratosclient.NewClient(kcfg.PublicURL, kcfg.AdminURL)
			users := repository.NewUserRepository(sharedDB)

			tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			fmt.Fprintln(tw, "IDENTITY ID\tUSERNAME\tEMAIL\tACTION")

			var purged, kept, skipped, failed int
			var pageToken string
			for {
				page, next, err := kc.ListIdentitiesPage(ctx, 250, pageToken)
				if err != nil {
					return fmt.Errorf("list identities: %w", err)
				}
				for _, id := range page {
					uname := id.Traits.Username
					email := id.Traits.Email

					// Has a panel user row → not an orphan, never touch.
					if _, uerr := users.FindByKratosIdentityID(ctx, id.ID); uerr == nil {
						kept++
						continue
					} else if !errors.Is(uerr, repository.ErrNotFound) {
						fmt.Fprintf(tw, "%s\t%s\t%s\tSKIP (lookup error: %v)\n", id.ID, uname, email, uerr)
						skipped++
						continue
					}

					// Orphan. Optionally scope to a single username/email.
					if onlyUsername != "" && uname != onlyUsername && email != onlyUsername {
						continue
					}

					if !apply {
						fmt.Fprintf(tw, "%s\t%s\t%s\twould-purge\n", id.ID, uname, email)
						purged++
						continue
					}
					if err := kc.DeleteIdentity(ctx, id.ID); err != nil {
						fmt.Fprintf(tw, "%s\t%s\t%s\tFAIL: %v\n", id.ID, uname, email, err)
						failed++
						continue
					}
					fmt.Fprintf(tw, "%s\t%s\t%s\tpurged\n", id.ID, uname, email)
					purged++
				}
				if next == "" {
					break
				}
				pageToken = next
			}
			tw.Flush()

			if apply {
				fmt.Printf("\nPurged %d orphan identities, kept %d, skipped %d, failed %d.\n", purged, kept, skipped, failed)
				if failed > 0 {
					return fmt.Errorf("%d identities failed to delete — re-run after fixing", failed)
				}
			} else {
				fmt.Printf("\n[dry-run] %d orphan identities would be purged (%d live kept, %d skipped). Re-run with --apply.\n", purged, kept, skipped)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&apply, "apply", false, "delete the orphan identities (default: dry-run)")
	cmd.Flags().StringVar(&onlyUsername, "username", "", "only purge the orphan whose username or email matches this value")
	return cmd
}
