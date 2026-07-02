// `jabali automation-token` cobra subcommands — list / mint / revoke.
//
// Brings the Automation API token surface (GUI/REST) to the root CLI for
// headless bootstrap + provisioning (Gitea #544). Mint mirrors the admin REST
// handler exactly: 32 random bytes -> hex plaintext -> ssokey.Seal -> stored
// SecretEnc, plaintext revealed ONCE. Scope validation reuses isAllowedAutoScope
// (the same allowlist the API enforces). Mutations are CLI-audited (#537).
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/ids"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/repository"
)

func newAutomationTokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "automation-token",
		Short: "Manage Automation API tokens (headless provisioning)",
	}
	cmd.AddCommand(newAutomationTokenListCmd(), newAutomationTokenMintCmd(), newAutomationTokenRevokeCmd())
	return cmd
}

func newAutomationTokenListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List automation tokens (never reveals secrets)",
		Args:    cobra.NoArgs,
		PreRunE: requireDB,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			toks, err := repository.NewAutomationTokenRepository(sharedDB).List(ctx)
			if err != nil {
				return fmt.Errorf("list automation tokens: %w", err)
			}
			if jsonOutput {
				return printJSON(toks)
			}
			if len(toks) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No automation tokens.")
				return nil
			}
			for _, t := range toks {
				status := "active"
				if t.RevokedAt != nil {
					status = "revoked"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s  %-24s  %-8s  scopes=%v\n", t.ID, t.Name, status, []string(t.Scopes))
			}
			return nil
		},
	}
}

func newAutomationTokenMintCmd() *cobra.Command {
	var scopes []string
	cmd := &cobra.Command{
		Use:     "mint <name>",
		Short:   "Mint an automation token; reveals the secret once",
		Args:    cobra.ExactArgs(1),
		PreRunE: requireDB,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if len(name) == 0 || len(name) > 100 {
				return fmt.Errorf("name must be 1–100 chars")
			}
			if len(scopes) == 0 {
				return fmt.Errorf("at least one --scope is required")
			}
			for _, s := range scopes {
				if !models.IsAllowedAutomationScope(s) {
					return fmt.Errorf("scope not allowed: %s", s)
				}
			}
			key := ssoKeyForCLI()
			if key == nil {
				return fmt.Errorf("SSO key unavailable — cannot seal the token secret (set sso.key_path / run on the panel host)")
			}
			secretRaw := make([]byte, 32)
			if _, err := rand.Read(secretRaw); err != nil {
				return fmt.Errorf("generate secret: %w", err)
			}
			plaintext := hex.EncodeToString(secretRaw)
			enc, err := key.Seal([]byte(plaintext))
			if err != nil {
				return fmt.Errorf("seal secret: %w", err)
			}
			tok := &models.AutomationToken{
				ID:        ids.NewULID(),
				Name:      name,
				Scopes:    models.AutomationScopes(scopes),
				SecretEnc: enc,
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			if err := repository.NewAutomationTokenRepository(sharedDB).Create(ctx, tok); err != nil {
				cliAuditErr(ctx, "automation_token.mint", "token", name, nil)
				return fmt.Errorf("create automation token: %w", err)
			}
			cliAuditOK(ctx, "automation_token.mint", "token", tok.ID, nil)
			if jsonOutput {
				return printJSON(map[string]any{"id": tok.ID, "name": tok.Name, "scopes": tok.Scopes, "secret": plaintext})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Minted automation token %s (id=%s)\n", tok.Name, tok.ID)
			fmt.Fprintf(cmd.OutOrStdout(), "Secret: %s\n", plaintext)
			fmt.Fprintln(cmd.OutOrStdout(), "(Shown once — store it now; the panel keeps only an encrypted copy.)")
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&scopes, "scope", nil, "grant a scope (repeatable), e.g. --scope read:status")
	return cmd
}

func newAutomationTokenRevokeCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "revoke <id-or-name>",
		Short:   "Revoke an automation token",
		Args:    cobra.ExactArgs(1),
		PreRunE: requireDB,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			repo := repository.NewAutomationTokenRepository(sharedDB)
			spec := args[0]
			tok, err := repo.FindByID(ctx, spec)
			if err != nil {
				// Not an ID — resolve by name.
				toks, lerr := repo.List(ctx)
				if lerr != nil {
					return fmt.Errorf("resolve token: %w", lerr)
				}
				for i := range toks {
					if toks[i].Name == spec {
						tok = &toks[i]
						break
					}
				}
				if tok == nil {
					return fmt.Errorf("no automation token with id or name %q", spec)
				}
			}
			if err := repo.Revoke(ctx, tok.ID); err != nil {
				cliAuditErr(ctx, "automation_token.revoke", "token", tok.ID, nil)
				return fmt.Errorf("revoke: %w", err)
			}
			cliAuditOK(ctx, "automation_token.revoke", "token", tok.ID, nil)
			fmt.Fprintf(os.Stdout, "Revoked automation token %s (%s)\n", tok.Name, tok.ID)
			return nil
		},
	}
}
