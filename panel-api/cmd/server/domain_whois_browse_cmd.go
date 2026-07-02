// Domain WHOIS + docroot browse CLI (Gitea #586). Same agent verbs as the GUI
// helpers (domain.whois, files.list under the docroot). + system ssh-key
// management (Gitea #592): system.sshkeys.list / .generate.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/repository"
)

func newDomainWhoisCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "whois <domain-name|domain-id>",
		Short:   "WHOIS lookup for a domain",
		Args:    cobra.ExactArgs(1),
		PreRunE: requireDBAndAgent,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			dom, err := resolveDomainCLI(ctx, args[0])
			if err != nil {
				return err
			}
			return csCall(ctx, "domain.whois", map[string]any{"domain": dom.Name})
		},
	}
}

func newDomainBrowseCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "browse <domain-name|domain-id> [relative-path]",
		Short:   "List a path under the domain's docroot (as the domain owner)",
		Args:    cobra.RangeArgs(1, 2),
		PreRunE: requireDBAndAgent,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			dom, err := resolveDomainCLI(ctx, args[0])
			if err != nil {
				return err
			}
			rel := ""
			if len(args) == 2 {
				rel = args[1]
			}
			rel = strings.TrimPrefix(strings.TrimSpace(rel), "/")
			abs := filepath.Join(dom.DocRoot, rel)
			// Defence-in-depth: never leave the docroot.
			if r, rerr := filepath.Rel(dom.DocRoot, abs); rerr != nil || strings.HasPrefix(r, "..") {
				return fmt.Errorf("path escapes docroot")
			}
			owner, err := repository.NewUserRepository(sharedDB).FindByID(ctx, dom.UserID)
			if err != nil || owner == nil || owner.Username == nil || *owner.Username == "" {
				return fmt.Errorf("domain owner has no linux account")
			}
			return csCall(ctx, "files.list", map[string]any{
				"user_id": dom.UserID, "username": *owner.Username, "path": abs,
			})
		},
	}
}

func newSystemSSHKeysCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "ssh-keys", Short: "System SSH keys for backup destinations (list/generate)"}
	cmd.AddCommand(
		csPassthroughCmd("list", "List system SSH keys", "system.sshkeys.list"),
		newSystemSSHKeyGenerateCmd(),
	)
	return cmd
}

func newSystemSSHKeyGenerateCmd() *cobra.Command {
	var name, keyType string
	cmd := &cobra.Command{
		Use:     "generate",
		Short:   "Generate a system SSH keypair (for sftp backup destinations)",
		Args:    cobra.NoArgs,
		PreRunE: requireDBAndAgent,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}
			if keyType == "" {
				keyType = "ed25519"
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			raw, err := sharedAgent.Call(ctx, "system.sshkeys.generate", map[string]any{"name": name, "type": keyType})
			if err != nil {
				return err
			}
			cliAuditOK(ctx, "system.sshkey_generate", "ssh_key", name, nil)
			os.Stdout.Write(raw)
			os.Stdout.Write([]byte{'\n'})
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "key name (required)")
	cmd.Flags().StringVar(&keyType, "type", "ed25519", "key type (ed25519|rsa)")
	return cmd
}
