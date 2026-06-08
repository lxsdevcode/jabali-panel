package userops

import (
	"context"
	"time"
)

// PurgeDomainMail destroys EVERY Stalwart registry account under a domain.
// Both user-delete paths — the REST handler (panel-api/internal/api/users.go)
// and the CLI deleteUserDirect (panel-api/cmd/server/cli_ops.go) — call this
// before they delete the domain row, so neither leaks a Stalwart orphan.
//
// It MUST run BEFORE the domain row is deleted: the domain has to still be
// registered in Stalwart for the agent to resolve its accounts. Once the
// panel domain row goes, its mailbox rows FK-cascade away and the link to
// Stalwart is lost.
//
// Purge BY DOMAIN, not per panel mailbox row: a row-driven loop skips orphans
// (a migration that pushed mail to Stalwart without a matching row, or a
// failed prior delete that removed the row but not the account) — which is
// exactly how the orphan kept surviving. A domain belongs to one user, so
// destroying all its accounts on that user's delete is correct. The earlier
// CLI bug: jabali user delete cascaded domains without this call, leaving the
// Stalwart account behind → re-migrating the same address collided with
// {"type":"primaryKeyViolation","properties":["email"]}.
//
// Best-effort: a nil agent or empty domain is a no-op; the caller logs the
// returned error and continues (a recoverable orphan beats a blocked delete).
func PurgeDomainMail(ctx context.Context, agent AgentCaller, domain string) error {
	if agent == nil || domain == "" {
		return nil
	}
	mctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	_, err := agent.Call(mctx, "mail.domain.purge_accounts", map[string]any{
		"domain": domain,
	})
	return err
}
