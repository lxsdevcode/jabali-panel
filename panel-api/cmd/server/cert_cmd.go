// Certificate lifecycle CLI (Gitea #578 panel cert, #584 domain mail cert).
//
// Both surfaces are reconcile-coupled and faithful as plain DB writes: the live
// panel-api reconciler runs the ACME state machine every tick
// (reconciler/panel_certificate_reconciler.go for the panel hostname/mail certs,
// the mail-cert reconciler for per-domain certs). A CLI repo write — flip UseLE,
// set status to pending — converges to the same issuance the GUI triggers on the
// next pass (≤60s). The GUI's inline kick is only an immediacy optimization, not
// a correctness requirement (see the reconcile-apply pattern). Mutations audited
// (#537); routability/agent issuance stays owned by the one reconciler path so
// the CLI never reimplements the privileged ACME dispatch.
package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/models"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/repository"
)

func panelCertRepoFromDB() repository.PanelCertificateRepository {
	return repository.NewPanelCertificateRepository(sharedDB)
}

func mailCertRepoFromDB() repository.MailCertificateRepository {
	return repository.NewMailCertificateRepository(sharedDB)
}

func fmtTime(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return t.UTC().Format("2006-01-02 15:04Z")
}

// ---- panel cert (#578) ----

func newPanelCertCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "panel-cert",
		Short: "Manage the panel's own TLS certificates (hostname + mail)",
	}
	cmd.AddCommand(newPanelCertStatusCmd(), newPanelCertToggleCmd(), newPanelCertIssueCmd())
	return cmd
}

func panelCertKindArg(args []string) (string, error) {
	kind := models.PanelCertKindHostname
	if len(args) == 1 {
		kind = args[0]
	}
	if kind != models.PanelCertKindHostname && kind != models.PanelCertKindMail {
		return "", fmt.Errorf("kind must be %q or %q", models.PanelCertKindHostname, models.PanelCertKindMail)
	}
	return kind, nil
}

func newPanelCertStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "status",
		Short:   "Show both panel certificates (hostname + mail)",
		Args:    cobra.NoArgs,
		PreRunE: requireDB,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()
			rows, err := panelCertRepoFromDB().ListAll(ctx)
			if err != nil {
				return fmt.Errorf("load panel certificates: %w", err)
			}
			if jsonOutput {
				return printJSON(rows)
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "KIND\tHOSTNAME\tUSE_LE\tSTAGING\tSTATUS\tEXPIRES\tLAST_ERROR")
			for _, r := range rows {
				fmt.Fprintf(w, "%s\t%s\t%v\t%v\t%s\t%s\t%s\n",
					r.Kind, r.Hostname, r.UseLE, r.Staging, r.Status, fmtTime(r.ExpiresAt), truncErr(r.LastError))
			}
			return w.Flush()
		},
	}
}

func truncErr(s string) string {
	if s == "" {
		return "-"
	}
	if len(s) > 60 {
		return s[:57] + "..."
	}
	return s
}

func newPanelCertToggleCmd() *cobra.Command {
	var leFlag, stagingFlag string
	cmd := &cobra.Command{
		Use:   "toggle [hostname|mail]",
		Short: "Enable/disable Let's Encrypt (or staging) for a panel certificate",
		Long: "Flip use-le / staging on the hostname (default) or mail panel cert. " +
			"The reconciler issues or self-signs on the next pass (≤60s).",
		Args:    cobra.MaximumNArgs(1),
		PreRunE: requireDB,
		RunE: func(cmd *cobra.Command, args []string) error {
			if leFlag == "" && stagingFlag == "" {
				return fmt.Errorf("nothing to change: pass --le and/or --staging")
			}
			kind, err := panelCertKindArg(args)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()
			repo := panelCertRepoFromDB()
			row, err := repo.GetByKind(ctx, kind)
			if err != nil {
				return fmt.Errorf("load %s panel cert: %w", kind, err)
			}
			if leFlag != "" {
				v, perr := parseBool(leFlag)
				if perr != nil {
					return fmt.Errorf("--le: %w", perr)
				}
				row.UseLE = v
				// Turning LE off while a retry is parked → drop back to
				// self-signed, mirroring the REST toggle handler.
				if !v && row.Status == models.PanelCertStatusPendingACMERetry {
					row.Status = models.PanelCertStatusSelfSigned
					row.NextRetryAt = nil
				}
			}
			if stagingFlag != "" {
				v, perr := parseBool(stagingFlag)
				if perr != nil {
					return fmt.Errorf("--staging: %w", perr)
				}
				row.Staging = v
			}
			if err := repo.Upsert(ctx, row); err != nil {
				cliAuditErr(ctx, "panel_cert.toggle", "panel_certificate", kind, nil)
				return fmt.Errorf("save panel cert: %w", err)
			}
			cliAuditOK(ctx, "panel_cert.toggle", "panel_certificate", kind, nil)
			fmt.Printf("%s panel cert: use_le=%v staging=%v (reconciler applies on the next pass, ≤60s)\n", kind, row.UseLE, row.Staging)
			return nil
		},
	}
	cmd.Flags().StringVar(&leFlag, "le", "", "enable Let's Encrypt issuance (true|false)")
	cmd.Flags().StringVar(&stagingFlag, "staging", "", "use the LE staging environment (true|false)")
	return cmd
}

func newPanelCertIssueCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "issue [hostname|mail]",
		Short: "Request (re)issuance of a panel certificate on the next reconcile pass",
		Long: "Flips the cert to pending_acme so the reconciler issues it on the next " +
			"tick (≤60s). Requires use-le already enabled (run `toggle --le=true` first); " +
			"routability + ACME stay owned by the reconciler.",
		Args:    cobra.MaximumNArgs(1),
		PreRunE: requireDB,
		RunE: func(cmd *cobra.Command, args []string) error {
			kind, err := panelCertKindArg(args)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()
			repo := panelCertRepoFromDB()
			row, err := repo.GetByKind(ctx, kind)
			if err != nil {
				return fmt.Errorf("load %s panel cert: %w", kind, err)
			}
			if !row.UseLE {
				return fmt.Errorf("%s panel cert has Let's Encrypt disabled — run `jabali panel-cert toggle %s --le=true` first", kind, kind)
			}
			row.Status = models.PanelCertStatusPendingACME
			row.NextRetryAt = nil
			if err := repo.Upsert(ctx, row); err != nil {
				cliAuditErr(ctx, "panel_cert.issue", "panel_certificate", kind, nil)
				return fmt.Errorf("save panel cert: %w", err)
			}
			cliAuditOK(ctx, "panel_cert.issue", "panel_certificate", kind, nil)
			fmt.Printf("%s panel cert queued for issuance (status=pending_acme; reconciler issues on the next pass, ≤60s)\n", kind)
			return nil
		},
	}
}

// ---- domain mail cert (#584) ----

func newMailCertCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mail-cert",
		Short: "Manage a domain's mail TLS certificate (mail.<domain> SAN)",
	}
	cmd.AddCommand(newMailCertShowCmd(), newMailCertEnableCmd(), newMailCertDisableCmd(), newMailCertReissueCmd())
	return cmd
}

func newMailCertShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "show <domain-name|domain-id>",
		Short:   "Show a domain's mail certificate state",
		Args:    cobra.ExactArgs(1),
		PreRunE: requireDB,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()
			dom, err := resolveDomainCLI(ctx, args[0])
			if err != nil {
				return err
			}
			row, err := mailCertRepoFromDB().GetByDomain(ctx, dom.ID)
			if err != nil || row == nil {
				return fmt.Errorf("no mail certificate for %s (enable it first)", dom.Name)
			}
			if jsonOutput {
				return printJSON(row)
			}
			last := "-"
			if row.LastError != nil {
				last = truncErr(*row.LastError)
			}
			fmt.Printf("domain:     %s\nstatus:     %s\nissued_at:  %s\nexpires_at: %s\nattempts:   %d\nlast_error: %s\n",
				dom.Name, row.Status, fmtTime(row.IssuedAt), fmtTime(row.ExpiresAt), row.AttemptCount, last)
			return nil
		},
	}
}

func newMailCertEnableCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "enable <domain-name|domain-id>",
		Short:   "Enable the mail certificate for a domain (issues on the next reconcile pass)",
		Args:    cobra.ExactArgs(1),
		PreRunE: requireDB,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()
			dom, err := resolveDomainCLI(ctx, args[0])
			if err != nil {
				return err
			}
			repo := mailCertRepoFromDB()
			row, err := repo.EnsureForDomain(ctx, dom.ID)
			if err != nil {
				cliAuditErr(ctx, "mail_cert.enable", "domain", dom.ID, nil)
				return fmt.Errorf("ensure mail cert: %w", err)
			}
			// Re-arm a disabled/failed row so the reconciler picks it up.
			if row.Status == models.MailCertStatusDisabled || row.Status == models.MailCertStatusFailed {
				if err := repo.UpdateStatus(ctx, row.ID, models.MailCertStatusPending, nil); err != nil {
					cliAuditErr(ctx, "mail_cert.enable", "domain", dom.ID, nil)
					return fmt.Errorf("arm mail cert: %w", err)
				}
			}
			cliAuditOK(ctx, "mail_cert.enable", "domain", dom.ID, nil)
			fmt.Printf("mail cert enabled for %s (issues on the next reconcile pass, ≤60s)\n", dom.Name)
			return nil
		},
	}
}

func newMailCertDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "disable <domain-name|domain-id>",
		Short:   "Disable the mail certificate for a domain",
		Args:    cobra.ExactArgs(1),
		PreRunE: requireDB,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()
			dom, err := resolveDomainCLI(ctx, args[0])
			if err != nil {
				return err
			}
			repo := mailCertRepoFromDB()
			row, err := repo.GetByDomain(ctx, dom.ID)
			if err != nil || row == nil {
				return fmt.Errorf("no mail certificate for %s", dom.Name)
			}
			if err := repo.UpdateStatus(ctx, row.ID, models.MailCertStatusDisabled, nil); err != nil {
				cliAuditErr(ctx, "mail_cert.disable", "domain", dom.ID, nil)
				return fmt.Errorf("disable mail cert: %w", err)
			}
			cliAuditOK(ctx, "mail_cert.disable", "domain", dom.ID, nil)
			fmt.Printf("mail cert disabled for %s\n", dom.Name)
			return nil
		},
	}
}

func newMailCertReissueCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "reissue <domain-name|domain-id>",
		Short:   "Clear backoff and re-issue a domain's mail certificate",
		Args:    cobra.ExactArgs(1),
		PreRunE: requireDB,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()
			dom, err := resolveDomainCLI(ctx, args[0])
			if err != nil {
				return err
			}
			repo := mailCertRepoFromDB()
			row, err := repo.GetByDomain(ctx, dom.ID)
			if err != nil || row == nil {
				return fmt.Errorf("no mail certificate for %s (enable it first)", dom.Name)
			}
			if err := repo.UpdateStatus(ctx, row.ID, models.MailCertStatusPending, nil); err != nil {
				cliAuditErr(ctx, "mail_cert.reissue", "domain", dom.ID, nil)
				return fmt.Errorf("reissue mail cert: %w", err)
			}
			cliAuditOK(ctx, "mail_cert.reissue", "domain", dom.ID, nil)
			fmt.Printf("mail cert re-queued for %s (reconciler re-issues on the next pass, ≤60s)\n", dom.Name)
			return nil
		},
	}
}
