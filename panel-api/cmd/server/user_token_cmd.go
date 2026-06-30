// Per-user API token CLI (Gitea #570). Operator-side mirror of the
// /me/api-tokens REST surface (api/me_api_tokens.go): mint (plaintext shown
// once), list, and revoke a user's API tokens.
//
// The mint is byte-for-byte faithful to the handler: same `jat_` prefix, same
// 32-byte (256-bit) base64url secret, same SHA-256 hex hash + last-4 stored,
// same closed scope catalog via api.ValidateUserScopes (no drift), same 1-year
// expiry cap. No agent/reconciler — tokens are a pure DB row. Operator mutations
// are CLI-audited with the token owner as the subject user (#537).
//
// User-approved override of the break-glass-CLI stance for this specific
// existing-audited-mint-endpoint parity (see feedback_no_breakglass_cli memory).
package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/api"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/ids"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/models"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/repository"
)

func userAPITokenRepoFromDB() repository.UserAPITokenRepository {
	return repository.NewUserAPITokenRepository(sharedDB)
}

func newUserTokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user-token",
		Short: "Manage a user's API tokens (mint / list / revoke)",
	}
	cmd.AddCommand(newUserTokenListCmd(), newUserTokenMintCmd(), newUserTokenRevokeCmd())
	return cmd
}

func newUserTokenListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list <user-email|username|id>",
		Short:   "List a user's API tokens (active + revoked)",
		Args:    cobra.ExactArgs(1),
		PreRunE: requireDB,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()
			u, err := resolveUser(ctx, args[0])
			if err != nil {
				return err
			}
			rows, err := userAPITokenRepoFromDB().ListByUser(ctx, u.ID)
			if err != nil {
				return fmt.Errorf("list tokens: %w", err)
			}
			if jsonOutput {
				return printJSON(rows)
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tSECRET\tSCOPES\tEXPIRES\tSTATUS")
			now := time.Now().UTC()
			for _, t := range rows {
				status := "active"
				if t.RevokedAt != nil {
					status = "revoked"
				} else if t.ExpiresAt != nil && !now.Before(*t.ExpiresAt) {
					status = "expired"
				}
				scopes := "(full)"
				if len(t.Scopes) > 0 {
					scopes = strings.Join(t.Scopes, ",")
				}
				fmt.Fprintf(w, "%s\t%s\t…%s\t%s\t%s\t%s\n", t.ID, t.Name, t.SecretLast4, scopes, fmtTime(t.ExpiresAt), status)
			}
			return w.Flush()
		},
	}
}

func newUserTokenMintCmd() *cobra.Command {
	var name string
	var scopes []string
	var expiresIn time.Duration
	cmd := &cobra.Command{
		Use:   "mint <user-email|username|id> --name <name> [--scope read:dns ...]",
		Short: "Mint a new API token for a user (secret shown once)",
		Long: "Mint a per-user API token. The plaintext secret is printed ONCE and " +
			"never recoverable. An empty scope set = full owner privileges (same as " +
			"the panel UI); pass --scope to narrow. Expiry is capped at 1 year.",
		Args:    cobra.ExactArgs(1),
		PreRunE: requireDB,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()
			name = strings.TrimSpace(name)
			if name == "" || len(name) > 100 {
				return fmt.Errorf("--name is required, max 100 chars")
			}
			u, err := resolveUser(ctx, args[0])
			if err != nil {
				return err
			}
			scopeSet := models.UserAPIScopes(scopes)
			if bad, ok := api.ValidateUserScopes(scopeSet); !ok {
				return fmt.Errorf("unknown scope: %s", bad)
			}
			var expiresAt *time.Time
			if expiresIn > 0 {
				const maxLifetime = 365 * 24 * time.Hour
				if expiresIn > maxLifetime {
					return fmt.Errorf("--expires-in is capped at 1 year (8760h)")
				}
				t := time.Now().UTC().Add(expiresIn)
				expiresAt = &t
			}
			// Same secret shape as the REST mint: jat_ + 256-bit base64url.
			raw := make([]byte, 32)
			if _, err := rand.Read(raw); err != nil {
				return fmt.Errorf("rng: %w", err)
			}
			secret := "jat_" + base64.RawURLEncoding.EncodeToString(raw)
			sum := sha256.Sum256([]byte(secret))
			tok := &models.UserAPIToken{
				ID:          ids.NewULID(),
				UserID:      u.ID,
				Name:        name,
				SecretHash:  hex.EncodeToString(sum[:]),
				SecretLast4: secret[len(secret)-4:],
				Scopes:      scopeSet,
				ExpiresAt:   expiresAt,
			}
			if err := userAPITokenRepoFromDB().Create(ctx, tok); err != nil {
				cliAuditErr(ctx, "user_token.mint", "user_api_token", tok.ID, &u.ID)
				return fmt.Errorf("create token: %w", err)
			}
			cliAuditOK(ctx, "user_token.mint", "user_api_token", tok.ID, &u.ID)
			if jsonOutput {
				return printJSON(map[string]any{"id": tok.ID, "name": tok.Name, "secret": secret, "expires_at": tok.ExpiresAt})
			}
			fmt.Printf("Minted token %q for %s\nSecret: %s\n(Shown once — store it now; only a hash is kept.)\n", tok.Name, u.Email, secret)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "token name (required)")
	cmd.Flags().StringArrayVar(&scopes, "scope", nil, "scope to grant (repeatable; empty = full owner access)")
	cmd.Flags().DurationVar(&expiresIn, "expires-in", 0, "token lifetime (e.g. 720h); 0 = no expiry; max 8760h")
	return cmd
}

func newUserTokenRevokeCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "revoke <token-id>",
		Short:   "Revoke a user's API token",
		Args:    cobra.ExactArgs(1),
		PreRunE: requireDB,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()
			repo := userAPITokenRepoFromDB()
			tok, err := repo.FindByID(ctx, args[0])
			if err != nil || tok == nil {
				return fmt.Errorf("no token with id %q", args[0])
			}
			if err := repo.Revoke(ctx, tok.ID, time.Now().UTC()); err != nil {
				cliAuditErr(ctx, "user_token.revoke", "user_api_token", tok.ID, &tok.UserID)
				return fmt.Errorf("revoke token: %w", err)
			}
			cliAuditOK(ctx, "user_token.revoke", "user_api_token", tok.ID, &tok.UserID)
			fmt.Printf("revoked token %s (%s)\n", tok.ID, tok.Name)
			return nil
		},
	}
}
