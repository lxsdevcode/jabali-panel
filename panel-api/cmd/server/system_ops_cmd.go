// System + DB lifecycle CLI parity (Gitea #580/#581/#566/#593). Thin wrappers
// over the same agent verbs the admin REST handlers use:
//   system resolver get/set   → system.resolver.get / .set  (#580)
//   system diagnostic         → system.diagnostic_report     (#581)
//   system process list/kill  → system.processes / kill_process (#566)
//   db postgres status/uninstall → db.postgres.status / .uninstall (#593)
// Destructive ops (process kill, postgres uninstall) are --force gated and
// CLI-audited; inventory honours --json.
package main

import (
	"context"
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// ---- #580 system resolver ----

func newSystemResolverCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "resolver", Short: "Show / set system DNS resolvers"}
	cmd.AddCommand(
		csPassthroughCmd("get", "Show current resolver source + addresses", "system.resolver.get"),
		newSystemResolverSetCmd(),
	)
	return cmd
}

func newSystemResolverSetCmd() *cobra.Command {
	var addrs []string
	var search string
	cmd := &cobra.Command{
		Use:     "set",
		Short:   "Set resolver addresses (validated; same rules as the UI)",
		Args:    cobra.NoArgs,
		PreRunE: requireDBAndAgent,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cleaned := make([]string, 0, len(addrs))
			seen := map[string]bool{}
			for _, r := range addrs {
				r = strings.TrimSpace(r)
				if r == "" {
					continue
				}
				addr, err := netip.ParseAddr(r)
				if err != nil {
					return fmt.Errorf("not a valid IP: %s", r)
				}
				if addr.IsUnspecified() || addr.IsMulticast() {
					return fmt.Errorf("%s is not a usable unicast address", r)
				}
				if seen[r] {
					return fmt.Errorf("duplicate resolver: %s", r)
				}
				seen[r] = true
				cleaned = append(cleaned, r)
			}
			if len(cleaned) == 0 {
				return fmt.Errorf("at least one --addr resolver is required")
			}
			if len(cleaned) > 8 {
				return fmt.Errorf("at most 8 resolvers allowed")
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
			defer cancel()
			err := csCall(ctx, "system.resolver.set", map[string]any{
				"resolvers": cleaned, "search_domain": strings.TrimSpace(search),
			})
			if err == nil {
				cliAuditOK(ctx, "system.resolver_set", "system", "resolvers", nil)
			}
			return err
		},
	}
	cmd.Flags().StringArrayVar(&addrs, "addr", nil, "resolver IP (repeatable, IPv4 or IPv6)")
	cmd.Flags().StringVar(&search, "search-domain", "", "optional search domain")
	return cmd
}

// ---- #581 support diagnostic ----

func newSystemDiagnosticCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "diagnostic",
		Short:   "Generate a redacted, encrypted diagnostic bundle for support handoff",
		Long:    "Collects + redacts host diagnostics and uploads an encrypted bundle. Prints the share URL and one-time password — hand them to support. The bundle is encrypted; the CLI never emails anything.",
		Args:    cobra.NoArgs,
		PreRunE: requireDBAndAgent,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
			defer cancel()
			raw, err := sharedAgent.Call(ctx, "system.diagnostic_report", nil)
			if err != nil {
				return fmt.Errorf("diagnostic report (agent timeout/upload failure?): %w", err)
			}
			cliAuditOK(ctx, "system.diagnostic_report", "system", "diagnostic", nil)
			os.Stdout.Write(raw)
			os.Stdout.Write([]byte{'\n'})
			return nil
		},
	}
}

// ---- #566 admin process kill ----

func newSystemProcessCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "process", Short: "List / kill host processes (admin)"}
	cmd.AddCommand(
		csPassthroughCmd("list", "List host processes", "system.processes"),
		newSystemProcessKillCmd(),
	)
	return cmd
}

func newSystemProcessKillCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:     "kill <pid>",
		Short:   "Kill a process (TERM, or KILL with --force)",
		Args:    cobra.ExactArgs(1),
		PreRunE: requireDBAndAgent,
		RunE: func(cmd *cobra.Command, args []string) error {
			pid, err := strconv.Atoi(args[0])
			if err != nil || pid < 2 {
				return fmt.Errorf("pid must be an integer > 1")
			}
			if !force {
				return fmt.Errorf("killing pid %d sends SIGTERM (or SIGKILL with --force) — re-run with --force to confirm", pid)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()
			err = csCall(ctx, "system.kill_process", map[string]any{"pid": pid, "force": force})
			if err == nil {
				cliAuditOK(ctx, "system.process_kill", "process", args[0], nil)
			} else {
				cliAuditErr(ctx, "system.process_kill", "process", args[0], nil)
			}
			return err
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "confirm the kill (and use SIGKILL)")
	return cmd
}

// ---- #593 postgres lifecycle ----

func newDBPostgresCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "postgres", Short: "PostgreSQL lifecycle: status / uninstall"}
	cmd.AddCommand(
		csPassthroughCmd("status", "Show PostgreSQL installed/active/version", "db.postgres.status"),
		newDBPostgresUninstallCmd(),
	)
	return cmd
}

func newDBPostgresUninstallCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "DESTROY PostgreSQL: purge packages + delete all data dirs",
		Long: "Destructive: purges the PostgreSQL packages and removes /var/lib/postgresql + /etc/postgresql (ALL databases gone). This is NOT 'disable' — to stop/disable reversibly, set postgres_enabled=false via `jabali settings`. Requires --force.",
		Args:    cobra.NoArgs,
		PreRunE: requireDBAndAgent,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !force {
				return fmt.Errorf("uninstall PERMANENTLY deletes all PostgreSQL data — re-run with --force (to merely stop it, use `jabali settings set postgres_enabled=false`)")
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
			defer cancel()
			raw, err := sharedAgent.Call(ctx, "db.postgres.uninstall", map[string]any{})
			if err != nil {
				cliAuditErr(ctx, "database.postgres_uninstall", "database_engine", "postgres", nil)
				return fmt.Errorf("agent db.postgres.uninstall: %w", err)
			}
			cliAuditOK(ctx, "database.postgres_uninstall", "database_engine", "postgres", nil)
			os.Stdout.Write(raw)
			os.Stdout.Write([]byte{'\n'})
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "confirm permanent destruction of all PostgreSQL data")
	return cmd
}
