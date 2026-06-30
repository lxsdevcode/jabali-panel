// Application scan CLI (Gitea #556). Mirrors POST /applications/scan
// (applications.go scan): runs the agent app.scan over a user's homedir and
// registers any discovered WordPress/Joomla/etc installs that aren't already
// tracked, with the same domain-ownership + existing-install guards. Clone and
// cache are intentionally NOT mirrored here — they are deep WP-specific
// orchestration (destination DB provisioning; Redis ACL + nginx page cache) and
// a faithful CLI duplication would drift on data-creating paths; tracked for a
// dedicated follow-up.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/spf13/cobra"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/models"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/repository"
)

func newAppScanCmd() *cobra.Command {
	var user, adminEmail string
	cmd := &cobra.Command{
		Use:     "scan",
		Short:   "Scan a user's homedir for unregistered apps and register them",
		Args:    cobra.NoArgs,
		PreRunE: requireDBAndAgent,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if user == "" {
				return fmt.Errorf("--user is required (id or username)")
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
			defer cancel()
			u, err := resolveTenantUser(ctx, user)
			if err != nil {
				return err
			}
			if u.Username == nil || *u.Username == "" {
				return fmt.Errorf("user %s has no Linux username", u.ID)
			}
			if adminEmail == "" {
				adminEmail = u.Email
			}
			raw, err := sharedAgent.Call(ctx, "app.scan", map[string]any{"username": *u.Username})
			if err != nil {
				return fmt.Errorf("agent app.scan: %w", err)
			}
			var resp struct {
				Hits []struct {
					Domain        string `json:"domain"`
					Subdirectory  string `json:"subdirectory"`
					AppType       string `json:"app_type"`
					Version       string `json:"version"`
					AdminUsername string `json:"admin_username"`
				} `json:"hits"`
			}
			if jErr := json.Unmarshal(raw, &resp); jErr != nil {
				return fmt.Errorf("decode agent reply: %w", jErr)
			}
			domRepo := domainRepoFromDB()
			instRepo := repository.NewApplicationInstallRepository(sharedDB)
			type rep struct {
				Domain, Subdirectory, AppType, Action, InstallID string
			}
			var report []rep
			added := 0
			for _, hit := range resp.Hits {
				dom, dErr := domRepo.FindByName(ctx, hit.Domain)
				if dErr != nil || dom == nil {
					report = append(report, rep{hit.Domain, hit.Subdirectory, hit.AppType, "skipped_no_domain", ""})
					continue
				}
				if dom.UserID != u.ID {
					report = append(report, rep{hit.Domain, hit.Subdirectory, hit.AppType, "skipped_domain_owned_by_other_user", ""})
					continue
				}
				if existing, _ := instRepo.FindByDomainAndSubdirectoryAndAppType(ctx, dom.ID, hit.Subdirectory, hit.AppType); existing != nil {
					report = append(report, rep{hit.Domain, hit.Subdirectory, hit.AppType, "skipped_already_present", existing.ID})
					continue
				}
				adminUser := hit.AdminUsername
				if adminUser == "" {
					adminUser = *u.Username
				}
				inst := &models.ApplicationInstall{
					ID: ulid.Make().String(), UserID: u.ID, DomainID: dom.ID,
					AdminUsername: adminUser, AdminEmail: adminEmail, Locale: "en_US",
					Subdirectory: hit.Subdirectory, Status: "ready", AppType: hit.AppType,
				}
				if hit.Version != "" {
					inst.Version = &hit.Version
				}
				if cErr := instRepo.Create(ctx, inst); cErr != nil {
					report = append(report, rep{hit.Domain, hit.Subdirectory, hit.AppType, "skipped_db_error", ""})
					continue
				}
				added++
				report = append(report, rep{hit.Domain, hit.Subdirectory, hit.AppType, "added", inst.ID})
			}
			cliAuditOK(ctx, "application.scan", "application_install", *u.Username, &u.ID)
			if jsonOutput {
				return printJSON(map[string]any{"scanned": len(resp.Hits), "added": added, "report": report})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "scanned=%d added=%d\n", len(resp.Hits), added)
			tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			fmt.Fprintln(tw, "DOMAIN\tSUBDIR\tTYPE\tACTION\tINSTALL_ID")
			for _, r := range report {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.Domain, r.Subdirectory, r.AppType, r.Action, r.InstallID)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&user, "user", "", "user (id or username) whose homedir to scan (required)")
	cmd.Flags().StringVar(&adminEmail, "admin-email", "", "admin email for discovered installs (default: user's email)")
	return cmd
}
