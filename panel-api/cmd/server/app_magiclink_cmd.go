// `jabali app magic-link` (Gitea #573). Operator-side mirror of the
// POST /applications/:id/magic-link REST endpoint (api/magic_link.go, ADR-0040).
//
// Faithful: same install resolution, same status=="ready" gate, the SAME
// supported-app map / install-path / URL composition / TTL via the exported
// api.SSOAgentCommandFor / api.ComposeInstallPath / api.ComposeSSOURL /
// api.SSOTTLSeconds (no drift), and the SAME agent `<cms>.create_sso_file`
// dispatch with the identical payload. The agent writes the self-deleting
// jabali-sso-<nonce>.php; we print the URL once and mark it sensitive.
//
// The link is a 60-second single-use secret — audited (#537) without the file
// name (a SHA prefix only, mirroring the endpoint's redacted log), so a working
// credential never lands in the audit trail.
//
// User-approved (2026-06-30) operator parity for an existing audited endpoint.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/api"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/repository"
)

func newAppMagicLinkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "magic-link <install-id>",
		Short: "Mint a one-time magic-link login for an installed app (sensitive, ~60s)",
		Long: "Mint a self-deleting SSO login link for a supported installed app " +
			"(WordPress / Drupal / Joomla). The URL is a single-use, ~60-second " +
			"secret — open it immediately and don't share or log it.",
		Args:    cobra.ExactArgs(1),
		PreRunE: requireDBAndAgent,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
			defer cancel()
			installID := args[0]

			installs := repository.NewApplicationInstallRepository(sharedDB)
			install, err := installs.FindByID(ctx, installID)
			if err != nil || install == nil {
				return fmt.Errorf("no app install with id %q", installID)
			}
			if install.Status != "ready" {
				return fmt.Errorf("install is in status %q (must be ready)", install.Status)
			}
			agentCommand, ok := api.SSOAgentCommandFor(install.AppType)
			if !ok {
				return fmt.Errorf("unsupported_app_type: magic-link login not implemented for app_type %q", install.AppType)
			}
			domain, err := repository.NewDomainRepository(sharedDB).FindByID(ctx, install.DomainID)
			if err != nil || domain == nil {
				return fmt.Errorf("domain lookup failed for install %s", installID)
			}
			if strings.TrimSpace(domain.DocRoot) == "" {
				return fmt.Errorf("domain %s has no docroot", domain.Name)
			}
			u, err := userRepo().FindByID(ctx, install.UserID)
			if err != nil || u == nil || u.Username == nil || *u.Username == "" {
				return fmt.Errorf("install owner has no linux username")
			}
			if install.AdminUsername == "" {
				return fmt.Errorf("install has no admin_username")
			}

			payload := map[string]any{
				"install_path":   api.ComposeInstallPath(domain.DocRoot, install.Subdirectory),
				"os_user":        *u.Username,
				"install_id":     install.ID,
				"admin_username": install.AdminUsername,
			}
			raw, err := sharedAgent.Call(ctx, agentCommand, payload)
			if err != nil {
				cliAuditErr(ctx, "app.magic_link", "application_install", install.ID, &install.UserID)
				return fmt.Errorf("agent %s: %w", agentCommand, err)
			}
			var resp struct {
				FileName      string `json:"file_name"`
				ExpiresAtUnix int64  `json:"expires_at_unix"`
			}
			if err := json.Unmarshal(raw, &resp); err != nil || resp.FileName == "" {
				cliAuditErr(ctx, "app.magic_link", "application_install", install.ID, &install.UserID)
				return fmt.Errorf("agent response invalid")
			}
			url := api.ComposeSSOURL(domain.Name, install.Subdirectory, resp.FileName)

			// Audit by install id only — the file name (a working credential)
			// is never written to the trail, mirroring the endpoint's redaction.
			cliAuditOK(ctx, "app.magic_link", "application_install", install.ID, &install.UserID)

			if jsonOutput {
				return printJSON(map[string]any{"url": url, "expires_in": api.SSOTTLSeconds})
			}
			fmt.Printf("Magic-link for %s on %s (SENSITIVE — single use, expires in %ds):\n%s\n",
				install.AppType, domain.Name, api.SSOTTLSeconds, url)
			return nil
		},
	}
}
