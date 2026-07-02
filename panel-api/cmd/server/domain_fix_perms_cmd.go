package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"git.jabali-panel.com/shukivaknin/jabali2/internal/fsperm"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/repository"
)

// newDomainFixPermsCmd — `jabali domain fix-perms <username>`.
//
// Remediation for docroots that ended up group-owned by the user instead of
// www-data (e.g. accounts imported by a migrator that didn't lay docroots down
// setgid). Retrofits every one of the user's domain docroots to group www-data
// + setgid on directories so nginx (www-data) can read files the per-user
// PHP-FPM pool creates. Equivalent to:
//
//	chgrp -R www-data <docroot> && find <docroot> -type d -exec chmod g+s {} +
//
// but symlink-safe (openat O_NOFOLLOW per component, anchored at /home/<user>).
func newDomainFixPermsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "fix-perms <username>",
		Short: "Repair docroot group/setgid (www-data) for all of a user's domains",
		Long: `Retrofits each of the user's domain docroots to group www-data +
setgid on directories, so nginx (www-data) can read files the per-user PHP-FPM
pool creates. Equivalent to:

    chgrp -R www-data <docroot> && find <docroot> -type d -exec chmod g+s {} +

but symlink-safe. Use after a migration that left docroots group-owned by the
user instead of www-data (idempotent — safe to re-run).`,
		Args:    cobra.ExactArgs(1),
		PreRunE: requireDB,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 120*time.Second)
			defer cancel()

			username := args[0]
			u, err := repository.NewUserRepository(sharedDB).FindByUsername(ctx, username)
			if err != nil || u == nil {
				return fmt.Errorf("user %q not found: %w", username, err)
			}
			gid, err := wwwDataGid()
			if err != nil {
				return fmt.Errorf("resolve www-data gid: %w", err)
			}
			domains, _, err := repository.NewDomainRepository(sharedDB).
				ListByUserID(ctx, u.ID, repository.ListOptions{Limit: 1000})
			if err != nil {
				return fmt.Errorf("list domains: %w", err)
			}
			if len(domains) == 0 {
				fmt.Printf("no domains for %s\n", username)
				return nil
			}

			fixed, failed := 0, 0
			for i := range domains {
				dr := domains[i].DocRoot
				home := userHomeOf(dr)
				if dr == "" || home == "" {
					fmt.Printf("  skip %s (docroot %q not under /home)\n", domains[i].Name, dr)
					continue
				}
				if err := fsperm.RepairDocrootGroup(home, dr, gid); err != nil {
					fmt.Printf("  FAIL %s (%s): %v\n", domains[i].Name, dr, err)
					failed++
					continue
				}
				fmt.Printf("  fixed %s (%s)\n", domains[i].Name, dr)
				fixed++
			}
			fmt.Printf("done: %d fixed, %d failed for %s\n", fixed, failed, username)
			if failed > 0 {
				return fmt.Errorf("%d docroot(s) failed", failed)
			}
			return nil
		},
	}
}
