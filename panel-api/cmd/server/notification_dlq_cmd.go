// `jabali notification dlq` (Gitea #563, second slice) — dead-letter queue
// operations, mirroring /admin/notifications/dlq (api/notifications_dlq.go).
//
// The DLQ is a Redis stream (jabali:notifications:dlq); these commands run the
// SAME XRANGE / XDEL / XADD primitives the REST handler uses — list, replay
// (re-publish to the main queue, stripping the reason/orig_id bookkeeping),
// replay-all, drop, and clear. The CLI builds its own Redis client from the
// panel.env socket + JABALI_REDIS_PANEL_TOKEN (the ADR-0148 ACL user), the same
// credentials panel-api authenticates with. Mutations CLI-audited (#537).
package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/notifications"
)

var sharedRedis *redis.Client

// requireRedis builds the CLI Redis client from panel.env (auto-loaded by the
// root PreRun), authenticating as the jabali_panel ACL user when the token is
// present — identical to serve.go's wiring.
func requireRedis(cmd *cobra.Command, _ []string) error {
	if sharedRedis != nil {
		return nil
	}
	// Load /etc/jabali/panel.env (DATABASE_URL, JABALI_REDIS_PANEL_TOKEN, …) into
	// the process env, and open the DB so DLQ mutations are CLI-audited (#537).
	if err := initConfig(); err != nil {
		return err
	}
	if err := initDB(); err != nil {
		return err
	}
	client, err := cliBuildRedis(cmd.Context())
	if err != nil {
		return err
	}
	sharedRedis = client
	return nil
}

// cliBuildRedis constructs a jabali_panel-authenticated Redis client from
// panel.env (the ADR-0148 ACL user), pinging to fail-fast. Shared by requireRedis
// (DLQ) and the app-install cfg (#597 default object cache). initConfig must have
// run first so the env + sharedCfg are loaded.
func cliBuildRedis(ctx context.Context) (*redis.Client, error) {
	url := "unix:///run/redis/redis.sock?db=0"
	if sharedCfg != nil && sharedCfg.Redis.URL != "" {
		url = sharedCfg.Redis.URL
	}
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("parse redis url %q: %w", url, err)
	}
	if tok := os.Getenv("JABALI_REDIS_PANEL_TOKEN"); tok != "" {
		opts.Username = "jabali_panel"
		opts.Password = tok
	}
	client := redis.NewClient(opts)
	pctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if perr := client.Ping(pctx).Err(); perr != nil {
		return nil, fmt.Errorf("redis ping: %w (is redis-server up?)", perr)
	}
	return client, nil
}

func newNotificationDLQCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dlq",
		Short: "Inspect and operate the notification dead-letter queue",
	}
	cmd.AddCommand(
		newDLQListCmd(), newDLQReplayCmd(), newDLQReplayAllCmd(), newDLQDropCmd(), newDLQClearCmd(),
	)
	return cmd
}

// streamIDToTime parses the millisecond prefix of a Redis stream ID.
func streamIDToTime(id string) string {
	var ms int64
	for i := 0; i < len(id); i++ {
		if id[i] == '-' {
			break
		}
		ms = ms*10 + int64(id[i]-'0')
	}
	if ms == 0 {
		return id
	}
	return time.UnixMilli(ms).UTC().Format("2006-01-02 15:04:05Z")
}

func newDLQListCmd() *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List dead-lettered notifications (newest first)",
		Args:    cobra.NoArgs,
		PreRunE: requireRedis,
		RunE: func(c *cobra.Command, _ []string) error {
			ctx := c.Context()
			total, _ := sharedRedis.XLen(ctx, notifications.StreamDLQ).Result()
			if limit <= 0 || limit > 500 {
				limit = 100
			}
			msgs, err := sharedRedis.XRevRangeN(ctx, notifications.StreamDLQ, "+", "-", int64(limit)).Result()
			if err != nil {
				return fmt.Errorf("read dlq: %w", err)
			}
			if jsonOutput {
				out := make([]map[string]any, 0, len(msgs))
				for _, m := range msgs {
					out = append(out, map[string]any{"id": m.ID, "at": streamIDToTime(m.ID), "values": m.Values})
				}
				return printJSON(map[string]any{"total": total, "entries": out})
			}
			fmt.Printf("DLQ total: %d\n", total)
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tAT\tEVENT\tREASON")
			for _, m := range msgs {
				fmt.Fprintf(w, "%s\t%s\t%v\t%v\n", m.ID, streamIDToTime(m.ID), m.Values["event_kind"], truncErr(fmt.Sprintf("%v", m.Values["reason"])))
			}
			return w.Flush()
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 100, "max entries to show (1-500)")
	return cmd
}

// replayOne re-publishes a DLQ entry to the main queue, stripping the
// reason/orig_id bookkeeping — identical to the REST handler. Returns
// (replayed, skippedLegacy, error).
func replayOne(ctx context.Context, m redis.XMessage) (bool, bool, error) {
	cleaned := map[string]any{}
	for k, v := range m.Values {
		if k == "reason" || k == "orig_id" {
			continue
		}
		cleaned[k] = v
	}
	if len(cleaned) == 0 {
		// Legacy entry with no preserved payload — drop it, can't replay.
		_ = sharedRedis.XDel(ctx, notifications.StreamDLQ, m.ID).Err()
		return false, true, nil
	}
	pipe := sharedRedis.TxPipeline()
	pipe.XAdd(ctx, &redis.XAddArgs{Stream: notifications.StreamQueue, Values: cleaned})
	pipe.XDel(ctx, notifications.StreamDLQ, m.ID)
	if _, err := pipe.Exec(ctx); err != nil {
		return false, false, err
	}
	return true, false, nil
}

func newDLQReplayCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "replay <id>",
		Short:   "Re-publish one DLQ entry to the main queue",
		Args:    cobra.ExactArgs(1),
		PreRunE: requireRedis,
		RunE: func(c *cobra.Command, args []string) error {
			ctx := c.Context()
			msgs, err := sharedRedis.XRange(ctx, notifications.StreamDLQ, args[0], args[0]).Result()
			if err != nil {
				return fmt.Errorf("xrange: %w", err)
			}
			if len(msgs) == 0 {
				return fmt.Errorf("no DLQ entry with id %q", args[0])
			}
			ok, legacy, rerr := replayOne(ctx, msgs[0])
			if rerr != nil {
				cliAuditErr(ctx, "notification_dlq.replay", "notification_dlq", args[0], nil)
				return rerr
			}
			if legacy {
				return fmt.Errorf("entry %q has no preserved payload (older dispatcher build) — dropped instead of replayed", args[0])
			}
			if ok {
				cliAuditOK(ctx, "notification_dlq.replay", "notification_dlq", args[0], nil)
				fmt.Printf("replayed %s\n", args[0])
			}
			return nil
		},
	}
	return cmd
}

func newDLQReplayAllCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "replay-all",
		Short:   "Re-publish every replayable DLQ entry to the main queue",
		Args:    cobra.NoArgs,
		PreRunE: requireRedis,
		RunE: func(c *cobra.Command, _ []string) error {
			ctx := c.Context()
			msgs, err := sharedRedis.XRange(ctx, notifications.StreamDLQ, "-", "+").Result()
			if err != nil && err != redis.Nil {
				return fmt.Errorf("xrange: %w", err)
			}
			var replayed, skipped int
			for _, m := range msgs {
				ok, legacy, rerr := replayOne(ctx, m)
				if rerr != nil {
					continue
				}
				if ok {
					replayed++
				} else if legacy {
					skipped++
				}
			}
			cliAuditOK(ctx, "notification_dlq.replay_all", "notification_dlq", "*", nil)
			fmt.Printf("replayed %d, skipped %d legacy entr%s\n", replayed, skipped, map[bool]string{true: "y", false: "ies"}[skipped == 1])
			return nil
		},
	}
	return cmd
}

func newDLQDropCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "drop <id>",
		Short:   "Delete one DLQ entry without replaying",
		Args:    cobra.ExactArgs(1),
		PreRunE: requireRedis,
		RunE: func(c *cobra.Command, args []string) error {
			ctx := c.Context()
			n, err := sharedRedis.XDel(ctx, notifications.StreamDLQ, args[0]).Result()
			if err != nil {
				cliAuditErr(ctx, "notification_dlq.drop", "notification_dlq", args[0], nil)
				return fmt.Errorf("xdel: %w", err)
			}
			if n == 0 {
				fmt.Printf("no DLQ entry with id %q\n", args[0])
				return nil
			}
			cliAuditOK(ctx, "notification_dlq.drop", "notification_dlq", args[0], nil)
			fmt.Printf("dropped %s\n", args[0])
			return nil
		},
	}
	return cmd
}

func newDLQClearCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:     "clear",
		Short:   "Delete ALL DLQ entries (does not replay)",
		Args:    cobra.NoArgs,
		PreRunE: requireRedis,
		RunE: func(c *cobra.Command, _ []string) error {
			if !force {
				return fmt.Errorf("refusing to clear the whole DLQ without --force")
			}
			ctx := c.Context()
			msgs, err := sharedRedis.XRange(ctx, notifications.StreamDLQ, "-", "+").Result()
			if err != nil && err != redis.Nil {
				return fmt.Errorf("xrange: %w", err)
			}
			var dropped int
			for _, m := range msgs {
				if sharedRedis.XDel(ctx, notifications.StreamDLQ, m.ID).Err() == nil {
					dropped++
				}
			}
			cliAuditOK(ctx, "notification_dlq.clear", "notification_dlq", "*", nil)
			fmt.Printf("cleared %d DLQ entr%s\n", dropped, map[bool]string{true: "y", false: "ies"}[dropped == 1])
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "confirm clearing the entire DLQ")
	return cmd
}
