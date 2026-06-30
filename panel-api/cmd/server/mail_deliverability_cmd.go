// `jabali mail deliverability` (Gitea #579). Operator-side mirror of
// GET /admin/mail/deliverability. It calls the SAME api.ComputeDeliverability
// the endpoint now delegates to (extracted in this change), wired with the same
// repos, so the CLI score + component breakdown are byte-identical to the GUI —
// no reimplementation, no drift. Read-only; no audit needed.
package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/api"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/repository"
)

func newMailDeliverabilityCmd() *cobra.Command {
	var domainRef string
	cmd := &cobra.Command{
		Use:   "deliverability",
		Short: "Show the mail deliverability score (server-wide or per-domain)",
		Long: "Aggregates RBL / DMARC / TLS-RPT / ARF signals into a 0-100 score " +
			"(100 = clean) with the per-signal deductions — the same computation " +
			"the admin Mail panel shows. Pass --domain to narrow DMARC/TLS-RPT/ARF.",
		Args:    cobra.NoArgs,
		PreRunE: requireDB,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 20*time.Second)
			defer cancel()
			domain := domainRef
			if domain != "" {
				// Validate/normalise the domain exists, like the GUI selector.
				dom, err := resolveDomainCLI(ctx, domain)
				if err != nil {
					return err
				}
				domain = dom.Name
			}
			cfg := api.AdminMailDeliverabilityHandlerConfig{
				MailRBLStates:   repository.NewMailRBLStateRepository(sharedDB),
				DMARCAggregate:  repository.NewDMARCAggregateRepository(sharedDB),
				TLSRPTAggregate: repository.NewTLSRPTAggregateRepository(sharedDB),
				ARFReports:      repository.NewARFReportRepository(sharedDB),
				ServerSettings:  repository.NewServerSettingsRepository(sharedDB),
			}
			res := api.ComputeDeliverability(ctx, cfg, domain, time.Now().UTC())
			if jsonOutput {
				return printJSON(res)
			}
			scope := "server-wide"
			if res.Domain != "" {
				scope = res.Domain
			}
			fmt.Printf("Deliverability: %d/100 (%s) — %s, generated %s\n",
				res.Score, res.Severity, scope, res.GeneratedAt.Format("2006-01-02 15:04Z"))
			if len(res.Components) == 0 {
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "SIGNAL\tVALUE\tDEDUCTION\tDETAIL")
			for _, comp := range res.Components {
				fmt.Fprintf(w, "%s\t%d\t-%d\t%s\n", comp.Name, comp.Value, comp.Deduction, comp.Detail)
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&domainRef, "domain", "", "scope to one domain (default: server-wide)")
	return cmd
}
