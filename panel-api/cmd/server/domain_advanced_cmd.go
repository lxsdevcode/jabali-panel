// Advanced domain settings CLI (Gitea #558). Ships the clean, fully-faithful
// slices: `domain show` (full advanced-settings state) and `domain ip-acl`
// CRUD (the per-domain IP allow/deny list the nginx snippet renders from).
//
// The reconcile-coupled mutation set — nginx options/rules, redirects, cache,
// directory index, SSL mode — is deliberately NOT shipped here: those apply via
// the in-process domain Reconciler (Schedule/ReconcileSSLInline), and a thin CLI
// repo-write would not reproduce the GUI's reconcile/reload behavior (criterion
// 6). Tracked for a dedicated follow-up that wires the reconcile path properly.
package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/ids"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/models"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/repository"
)

// resolveDomainCLI resolves a domain by ULID then by name.
func resolveDomainCLI(ctx context.Context, ref string) (*models.Domain, error) {
	repo := domainRepoFromDB()
	if d, err := repo.FindByID(ctx, ref); err == nil && d != nil {
		return d, nil
	}
	d, err := repo.FindByName(ctx, ref)
	if err != nil || d == nil {
		return nil, fmt.Errorf("no domain with id or name %q", ref)
	}
	return d, nil
}

func newDomainShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "show <domain-name|domain-id>",
		Short:   "Show a domain's full advanced-settings state (JSON)",
		Args:    cobra.ExactArgs(1),
		PreRunE: requireDB,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			d, err := resolveDomainCLI(ctx, args[0])
			if err != nil {
				return err
			}
			acls, _ := repository.NewDomainIPACLRepository(sharedDB).ListByDomain(ctx, d.ID)
			if jsonOutput {
				return printJSON(map[string]any{"domain": d, "ip_acls": acls})
			}
			return printJSON(map[string]any{"domain": d, "ip_acls": acls})
		},
	}
}

func newDomainIPACLCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "ip-acl", Short: "Manage a domain's IP allow/deny ACL"}
	cmd.AddCommand(newDomainIPACLListCmd(), newDomainIPACLAddCmd(), newDomainIPACLDeleteCmd())
	return cmd
}

func newDomainIPACLListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list <domain>",
		Short:   "List a domain's IP ACL entries",
		Args:    cobra.ExactArgs(1),
		PreRunE: requireDB,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()
			d, err := resolveDomainCLI(ctx, args[0])
			if err != nil {
				return err
			}
			rows, err := repository.NewDomainIPACLRepository(sharedDB).ListByDomain(ctx, d.ID)
			if err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(rows)
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tACTION\tCIDR\tPRIORITY\tCOMMENT")
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\n", r.ID, r.Action, r.CIDR, r.Priority, r.Comment)
			}
			return tw.Flush()
		},
	}
}

func newDomainIPACLAddCmd() *cobra.Command {
	var cidr, action, comment string
	var priority int
	cmd := &cobra.Command{
		Use:     "add <domain>",
		Short:   "Add an IP ACL entry to a domain",
		Args:    cobra.ExactArgs(1),
		PreRunE: requireDB,
		RunE: func(cmd *cobra.Command, args []string) error {
			if action != "allow" && action != "deny" {
				return fmt.Errorf("--action must be allow|deny")
			}
			if _, _, err := net.ParseCIDR(cidr); err != nil {
				if net.ParseIP(cidr) == nil {
					return fmt.Errorf("--cidr %q is not a valid IP or CIDR", cidr)
				}
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()
			d, err := resolveDomainCLI(ctx, args[0])
			if err != nil {
				return err
			}
			row := &models.DomainIPACL{ID: ids.NewULID(), DomainID: d.ID, CIDR: cidr, Action: action, Priority: priority, Comment: comment}
			if err := repository.NewDomainIPACLRepository(sharedDB).Create(ctx, row); err != nil {
				return fmt.Errorf("create ACL: %w", err)
			}
			cliAuditOK(ctx, "domain.ip_acl_add", "domain_ip_acl", row.ID, &d.UserID)
			fmt.Fprintf(cmd.OutOrStdout(), "Added %s %s (id=%s). Converges on the next domain reconcile.\n", action, cidr, row.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&cidr, "cidr", "", "IP or CIDR (required)")
	cmd.Flags().StringVar(&action, "action", "deny", "allow|deny")
	cmd.Flags().IntVar(&priority, "priority", 0, "rule priority (lower = first)")
	cmd.Flags().StringVar(&comment, "comment", "", "optional comment")
	_ = cmd.MarkFlagRequired("cidr")
	return cmd
}

func newDomainIPACLDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "delete <acl-id>",
		Short:   "Delete an IP ACL entry by id",
		Args:    cobra.ExactArgs(1),
		PreRunE: requireDB,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()
			if err := repository.NewDomainIPACLRepository(sharedDB).Delete(ctx, args[0]); err != nil {
				return fmt.Errorf("delete ACL: %w", err)
			}
			cliAuditOK(ctx, "domain.ip_acl_delete", "domain_ip_acl", args[0], nil)
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted ACL %s\n", args[0])
			return nil
		},
	}
}
