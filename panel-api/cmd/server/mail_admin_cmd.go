// Admin mail queue + throttle CLI (Gitea #560). Queue ops dispatch the same
// agent verbs as the API (mail_queue.go: mail.queue.list/retry/delete). Throttle
// ops are pure CRUD on mail_outbound_policy (admin_mail_throttles.go) — DB is the
// source of truth and the reconciler converges each row into Stalwart's
// MtaOutboundThrottle on the next tick, same as the GUI. Mutations CLI-audited;
// inventory honours --json.
package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/spf13/cobra"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/repository"
)

func newMailAdminCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mail",
		Short: "Admin mail queue + outbound throttle management",
	}
	cmd.AddCommand(newMailQueueCmd(), newMailThrottleCmd(), newMailLogsCmd(), newMailDeliverabilityCmd())
	return cmd
}

func newMailQueueCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "queue", Short: "Inspect / retry / delete queued mail"}
	cmd.AddCommand(
		csPassthroughCmd("list", "List queued mail", "mail.queue.list"),
		&cobra.Command{
			Use: "retry <id>", Short: "Retry a queued message", Args: cobra.ExactArgs(1), PreRunE: requireDBAndAgent,
			RunE: func(cmd *cobra.Command, args []string) error {
				ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
				defer cancel()
				err := csCall(ctx, "mail.queue.retry", map[string]any{"id": args[0]})
				if err == nil {
					cliAuditOK(ctx, "mail.queue_retry", "mail_queue", args[0], nil)
				}
				return err
			},
		},
		func() *cobra.Command {
			var force bool
			c := &cobra.Command{
				Use: "delete <id>", Short: "Delete a queued message", Args: cobra.ExactArgs(1), PreRunE: requireDBAndAgent,
				RunE: func(cmd *cobra.Command, args []string) error {
					if !force {
						return fmt.Errorf("deleting a queued message is irreversible; re-run with --force")
					}
					ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
					defer cancel()
					err := csCall(ctx, "mail.queue.delete", map[string]any{"id": args[0]})
					if err == nil {
						cliAuditOK(ctx, "mail.queue_delete", "mail_queue", args[0], nil)
					}
					return err
				},
			}
			c.Flags().BoolVar(&force, "force", false, "confirm deletion")
			return c
		}(),
	)
	return cmd
}

func validMailScope(s string) bool {
	return s == models.OutboundScopeUser || s == models.OutboundScopeDomain || s == models.OutboundScopeGlobal
}

func newMailThrottleCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "throttle", Short: "Outbound mail throttle policies"}
	cmd.AddCommand(newMailThrottleListCmd(), newMailThrottleCreateCmd(), newMailThrottleUpdateCmd(), newMailThrottleDeleteCmd())
	return cmd
}

func newMailThrottleListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List outbound throttle policies",
		Args:    cobra.NoArgs,
		PreRunE: requireDB,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()
			rows, err := repository.NewMailOutboundPolicyRepository(sharedDB).List(ctx)
			if err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(rows)
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tSCOPE\tSCOPE_REF\tPER_HOUR\tPER_DAY\tENABLED")
			for _, r := range rows {
				ref := ""
				if r.ScopeRef != nil {
					ref = *r.ScopeRef
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%d\t%v\n", r.ID, r.Scope, ref, r.MaxPerHour, r.MaxPerDay, r.Enabled)
			}
			return tw.Flush()
		},
	}
}

func newMailThrottleCreateCmd() *cobra.Command {
	var scope, scopeRef string
	var perHour, perDay uint
	var disabled bool
	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create an outbound throttle policy",
		Args:    cobra.NoArgs,
		PreRunE: requireDB,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !validMailScope(scope) {
				return fmt.Errorf("--scope must be user|domain|global")
			}
			var ref *string
			if scope == models.OutboundScopeGlobal {
				ref = nil
			} else {
				if scopeRef == "" {
					return fmt.Errorf("--scope-ref is required for scope=%s (email for user, FQDN for domain)", scope)
				}
				ref = &scopeRef
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()
			row := &models.MailOutboundPolicy{
				ID: ulid.Make().String(), Scope: scope, ScopeRef: ref,
				MaxPerHour: perHour, MaxPerDay: perDay, Enabled: !disabled,
			}
			if err := repository.NewMailOutboundPolicyRepository(sharedDB).Create(ctx, row); err != nil {
				return fmt.Errorf("create policy: %w", err)
			}
			cliAuditOK(ctx, "mail.throttle_create", "mail_outbound_policy", row.ID, nil)
			if jsonOutput {
				return printJSON(row)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created throttle %s (%s). Converges into Stalwart on the next reconcile tick.\n", row.ID, scope)
			return nil
		},
	}
	cmd.Flags().StringVar(&scope, "scope", "", "user|domain|global (required)")
	cmd.Flags().StringVar(&scopeRef, "scope-ref", "", "email (user) or FQDN (domain); omit for global")
	cmd.Flags().UintVar(&perHour, "max-per-hour", 0, "max messages/hour (0 = unlimited)")
	cmd.Flags().UintVar(&perDay, "max-per-day", 0, "max messages/day (0 = unlimited)")
	cmd.Flags().BoolVar(&disabled, "disabled", false, "create disabled")
	return cmd
}

func newMailThrottleUpdateCmd() *cobra.Command {
	var perHour, perDay uint
	var enable, disable bool
	cmd := &cobra.Command{
		Use:     "update <id>",
		Short:   "Update a throttle policy's limits / enabled flag",
		Args:    cobra.ExactArgs(1),
		PreRunE: requireDB,
		RunE: func(cmd *cobra.Command, args []string) error {
			if enable && disable {
				return fmt.Errorf("pass at most one of --enable / --disable")
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()
			repo := repository.NewMailOutboundPolicyRepository(sharedDB)
			row, err := repo.FindByID(ctx, args[0])
			if err != nil {
				return fmt.Errorf("policy not found: %w", err)
			}
			if cmd.Flags().Changed("max-per-hour") {
				row.MaxPerHour = perHour
			}
			if cmd.Flags().Changed("max-per-day") {
				row.MaxPerDay = perDay
			}
			if enable {
				row.Enabled = true
			}
			if disable {
				row.Enabled = false
			}
			if err := repo.Update(ctx, row); err != nil {
				return fmt.Errorf("update policy: %w", err)
			}
			cliAuditOK(ctx, "mail.throttle_update", "mail_outbound_policy", row.ID, nil)
			if jsonOutput {
				return printJSON(row)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated throttle %s\n", row.ID)
			return nil
		},
	}
	cmd.Flags().UintVar(&perHour, "max-per-hour", 0, "max messages/hour")
	cmd.Flags().UintVar(&perDay, "max-per-day", 0, "max messages/day")
	cmd.Flags().BoolVar(&enable, "enable", false, "enable the policy")
	cmd.Flags().BoolVar(&disable, "disable", false, "disable the policy")
	return cmd
}

func newMailThrottleDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "delete <id>",
		Short:   "Delete a throttle policy",
		Args:    cobra.ExactArgs(1),
		PreRunE: requireDB,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()
			if err := repository.NewMailOutboundPolicyRepository(sharedDB).Delete(ctx, args[0]); err != nil {
				return fmt.Errorf("delete policy: %w", err)
			}
			cliAuditOK(ctx, "mail.throttle_delete", "mail_outbound_policy", args[0], nil)
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted throttle %s. The reconciler removes its Stalwart row on the next tick.\n", args[0])
			return nil
		},
	}
}
