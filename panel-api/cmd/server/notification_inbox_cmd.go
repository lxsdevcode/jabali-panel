// `jabali notification inbox` (Gitea #563, inbox slice) — operator management of
// a user's notification bell inbox, mirroring /notifications/inbox
// (api/notifications_inbox.go): list, read one, read-all, clear.
// Same NotificationHistory repo the bell UI uses. Mutations CLI-audited (#537).
package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/repository"
)

func notificationHistoryRepoFromDB() repository.NotificationHistoryRepository {
	return repository.NewNotificationHistoryRepository(sharedDB)
}

func newNotificationInboxCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "inbox", Short: "Manage a user's notification bell inbox"}
	cmd.AddCommand(newInboxListCmd(), newInboxReadCmd(), newInboxReadAllCmd(), newInboxClearCmd())
	return cmd
}

func newInboxListCmd() *cobra.Command {
	var user string
	var limit int
	cmd := &cobra.Command{
		Use: "list --user <user>", Short: "List a user's recent notifications", Args: cobra.NoArgs, PreRunE: requireDB,
		RunE: func(c *cobra.Command, _ []string) error {
			if user == "" {
				return fmt.Errorf("--user is required")
			}
			ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
			defer cancel()
			u, err := resolveUser(ctx, user)
			if err != nil {
				return err
			}
			if limit <= 0 || limit > 200 {
				limit = 50
			}
			repo := notificationHistoryRepoFromDB()
			rows, total, err := repo.ListForUser(ctx, u.ID, repository.ListOptions{Limit: limit})
			if err != nil {
				return fmt.Errorf("list inbox: %w", err)
			}
			unread, _ := repo.CountUnreadForUser(ctx, u.ID)
			if jsonOutput {
				return printJSON(map[string]any{"total": total, "unread": unread, "rows": rows})
			}
			fmt.Printf("%s — %d total, %d unread\n", u.Email, total, unread)
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tWHEN\tSEV\tREAD\tTITLE")
			for i := range rows {
				r := rows[i]
				read := "•"
				if r.ReadAt != nil {
					read = "✓"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", r.ID, r.CreatedAt.UTC().Format("01-02 15:04"), r.Severity, read, r.Title)
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&user, "user", "", "target user (email|username|id, required)")
	cmd.Flags().IntVar(&limit, "limit", 50, "max rows (1-200)")
	return cmd
}

func newInboxReadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "read <notification-id>", Short: "Mark one notification read", Args: cobra.ExactArgs(1), PreRunE: requireDB,
		RunE: func(c *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
			defer cancel()
			if err := notificationHistoryRepoFromDB().MarkRead(ctx, args[0]); err != nil {
				cliAuditErr(ctx, "notification_inbox.read", "notification_history", args[0], nil)
				return fmt.Errorf("mark read: %w", err)
			}
			cliAuditOK(ctx, "notification_inbox.read", "notification_history", args[0], nil)
			fmt.Printf("marked %s read\n", args[0])
			return nil
		},
	}
	return cmd
}

func newInboxReadAllCmd() *cobra.Command {
	var user string
	cmd := &cobra.Command{
		Use: "read-all --user <user>", Short: "Mark all of a user's notifications read", Args: cobra.NoArgs, PreRunE: requireDB,
		RunE: func(c *cobra.Command, _ []string) error {
			if user == "" {
				return fmt.Errorf("--user is required")
			}
			ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
			defer cancel()
			u, err := resolveUser(ctx, user)
			if err != nil {
				return err
			}
			n, err := notificationHistoryRepoFromDB().MarkAllReadForUser(ctx, u.ID)
			if err != nil {
				cliAuditErr(ctx, "notification_inbox.read_all", "user", u.ID, &u.ID)
				return fmt.Errorf("mark all read: %w", err)
			}
			cliAuditOK(ctx, "notification_inbox.read_all", "user", u.ID, &u.ID)
			fmt.Printf("marked %d notification(s) read for %s\n", n, u.Email)
			return nil
		},
	}
	cmd.Flags().StringVar(&user, "user", "", "target user (required)")
	return cmd
}

func newInboxClearCmd() *cobra.Command {
	var user string
	var force, includeBroadcast bool
	cmd := &cobra.Command{
		Use: "clear --user <user>", Short: "Delete all of a user's notifications", Args: cobra.NoArgs, PreRunE: requireDB,
		RunE: func(c *cobra.Command, _ []string) error {
			if user == "" {
				return fmt.Errorf("--user is required")
			}
			if !force {
				return fmt.Errorf("refusing to clear the inbox without --force")
			}
			ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
			defer cancel()
			u, err := resolveUser(ctx, user)
			if err != nil {
				return err
			}
			n, err := notificationHistoryRepoFromDB().DeleteAllForUser(ctx, u.ID, includeBroadcast)
			if err != nil {
				cliAuditErr(ctx, "notification_inbox.clear", "user", u.ID, &u.ID)
				return fmt.Errorf("clear inbox: %w", err)
			}
			cliAuditOK(ctx, "notification_inbox.clear", "user", u.ID, &u.ID)
			fmt.Printf("cleared %d notification(s) for %s\n", n, u.Email)
			return nil
		},
	}
	cmd.Flags().StringVar(&user, "user", "", "target user (required)")
	cmd.Flags().BoolVar(&force, "force", false, "confirm clearing")
	cmd.Flags().BoolVar(&includeBroadcast, "include-broadcast", false, "also remove broadcast rows (admin)")
	return cmd
}
