// UFW management CLI parity with the Security UI/API (Gitea #550). Each verb
// dispatches the same agent command the REST handler does (security_ufw.go):
// status, rule add/delete, default policy, enable/disable. The agent is the
// authoritative layer (validation + apply). Lockout-prone actions (disable,
// default-deny incoming) require --force and print an SSH-lockout warning, since
// the CLI typically runs over the very SSH session the change could cut.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

func ufwAgentCall(ctx context.Context, verb string, params map[string]any) error {
	raw, err := sharedAgent.Call(ctx, verb, params)
	if err != nil {
		return err
	}
	os.Stdout.Write(raw)
	os.Stdout.Write([]byte{'\n'})
	return nil
}

func newUfwStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "status",
		Short:   "Show UFW status and numbered rules",
		Args:    cobra.NoArgs,
		PreRunE: requireDBAndAgent,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			raw, err := sharedAgent.Call(ctx, "security.ufw.status", nil)
			if err != nil {
				return err
			}
			if jsonOutput {
				os.Stdout.Write(raw)
				os.Stdout.Write([]byte{'\n'})
				return nil
			}
			var st struct {
				Active     bool   `json:"active"`
				DefaultIn  string `json:"default_in"`
				DefaultOut string `json:"default_out"`
				Rules      []struct {
					Num    int    `json:"num"`
					Action string `json:"action"`
					From   string `json:"from"`
					To     string `json:"to"`
				} `json:"rules"`
			}
			if jerr := json.Unmarshal(raw, &st); jerr != nil {
				os.Stdout.Write(raw)
				os.Stdout.Write([]byte{'\n'})
				return nil
			}
			active := "inactive"
			if st.Active {
				active = "active"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Status:  %s\nDefault: incoming=%s outgoing=%s\n\n", active, st.DefaultIn, st.DefaultOut)
			tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			fmt.Fprintln(tw, "NUM\tACTION\tTO\tFROM")
			for _, r := range st.Rules {
				fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n", r.Num, r.Action, r.To, r.From)
			}
			return tw.Flush()
		},
	}
}

func newUfwRuleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rule",
		Short: "Add or delete UFW rules",
	}
	cmd.AddCommand(newUfwRuleAddCmd(), newUfwRuleDeleteCmd())
	return cmd
}

func newUfwRuleAddCmd() *cobra.Command {
	var (
		action string
		port   string
		proto  string
		from   string
	)
	cmd := &cobra.Command{
		Use:     "add",
		Short:   "Add a UFW rule",
		Args:    cobra.NoArgs,
		PreRunE: requireDBAndAgent,
		RunE: func(cmd *cobra.Command, _ []string) error {
			switch action {
			case "allow", "deny", "reject", "limit":
			default:
				return fmt.Errorf("--action must be allow|deny|reject|limit (got %q)", action)
			}
			if port == "" {
				return fmt.Errorf("--port is required (e.g. 443, or 'OpenSSH')")
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			params := map[string]any{"action": action, "port": port}
			if proto != "" {
				params["proto"] = proto
			}
			if from != "" {
				params["from"] = from
			}
			return ufwAgentCall(ctx, "security.ufw.rule.add", params)
		},
	}
	cmd.Flags().StringVar(&action, "action", "allow", "allow|deny|reject|limit")
	cmd.Flags().StringVar(&port, "port", "", "port or app name (e.g. 443, OpenSSH) (required)")
	cmd.Flags().StringVar(&proto, "proto", "", "tcp|udp (optional)")
	cmd.Flags().StringVar(&from, "from", "", "source address/CIDR (optional)")
	return cmd
}

func newUfwRuleDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "delete <num>",
		Short:   "Delete a UFW rule by its numbered position (see `ufw status`)",
		Args:    cobra.ExactArgs(1),
		PreRunE: requireDBAndAgent,
		RunE: func(cmd *cobra.Command, args []string) error {
			num, err := strconv.Atoi(args[0])
			if err != nil || num < 1 {
				return fmt.Errorf("rule number must be a positive integer")
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			return ufwAgentCall(ctx, "security.ufw.rule.delete", map[string]any{"num": num})
		},
	}
}

func newUfwDefaultCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:     "default <incoming|outgoing> <allow|deny|reject>",
		Short:   "Set the default policy for a chain",
		Args:    cobra.ExactArgs(2),
		PreRunE: requireDBAndAgent,
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, policy := args[0], args[1]
			if chain != "incoming" && chain != "outgoing" {
				return fmt.Errorf("chain must be 'incoming' or 'outgoing' (got %q)", chain)
			}
			switch policy {
			case "allow", "deny", "reject":
			default:
				return fmt.Errorf("policy must be allow|deny|reject (got %q)", policy)
			}
			// Default-deny/reject on INCOMING can drop the SSH session if no
			// allow rule covers your port.
			if chain == "incoming" && (policy == "deny" || policy == "reject") && !force {
				return fmt.Errorf("setting default incoming %s can lock out your SSH/admin session if no allow rule covers your port; re-run with --force", policy)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			return ufwAgentCall(ctx, "security.ufw.default.set", map[string]any{"chain": chain, "policy": policy})
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "confirm a lockout-prone default policy")
	return cmd
}

func newUfwToggleCmd(verb, short string, lockoutRisk bool) *cobra.Command {
	var force bool
	action := "enable"
	if verb == "security.ufw.disable" {
		action = "disable"
	}
	cmd := &cobra.Command{
		Use:     action,
		Short:   short,
		Args:    cobra.NoArgs,
		PreRunE: requireDBAndAgent,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !force {
				warn := ""
				if lockoutRisk {
					warn = " (this can cut your SSH/admin session if the firewall rules don't allow your port)"
				}
				return fmt.Errorf("%s UFW is a destructive action%s; re-run with --force", action, warn)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			// The agent gate requires confirm:"YES" (mirrors the REST contract).
			return ufwAgentCall(ctx, verb, map[string]any{"confirm": "YES"})
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "confirm this destructive action")
	return cmd
}
