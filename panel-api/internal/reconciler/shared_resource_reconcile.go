package reconciler

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/models"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/repository"
)

// shared_resource_reconcile.go — M52 (ADR-0133). Converge shared_resources +
// their grants into Stalwart: each resource gets a host principal
// (sharedresource.apply, idempotent) and each resource's grants are pushed as
// the collection's shareWith via the per-kind *.share_set agent command (an
// idempotent whole-map replace). DB is truth.
//
// Calendar / address book / file folder are handled here (selective sharing,
// #242). The "mailbox" kind (#241) is deferred: a shared mailbox a member can
// SEND AS needs Stalwart group membership, not just shareWith — see the
// blueprint's Gate 2 / send-as section.

// sharedResourceShareCmd maps a resource kind to its share_set agent command.
// Absent kinds (mailbox) are skipped by the reconcile loop.
var sharedResourceShareCmd = map[string]string{
	"calendar":    "calendar.share_set",
	"addressbook": "addressbook.share_set",
	"files":       "file.share_set",
}

// WithSharedResources wires the M52 shared-resources convergence pass. All
// three repos are required; nil on any disables the pass.
func (r *Reconciler) WithSharedResources(
	sr repository.SharedResourceRepository,
	mailboxes repository.MailboxRepository,
	mailGroups repository.MailGroupRepository,
) *Reconciler {
	r.sharedResources = sr
	r.srMailboxes = mailboxes
	r.srMailGroups = mailGroups
	return r
}

func (r *Reconciler) reconcileSharedResources(ctx context.Context) {
	if r.sharedResources == nil || r.agent == nil || r.srMailboxes == nil || r.srMailGroups == nil {
		return
	}
	resources, err := r.sharedResources.ListAll(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "shared_resources: list", "err", err)
		return
	}
	for i := range resources {
		r.reconcileOneSharedResource(ctx, &resources[i])
	}
}

func (r *Reconciler) reconcileOneSharedResource(ctx context.Context, res *models.SharedResource) {
	cmd, ok := sharedResourceShareCmd[res.Kind]
	if !ok {
		return // mailbox kind deferred (send-as needs membership)
	}
	if res.EmailCached == nil || *res.EmailCached == "" {
		return // host address not assigned yet
	}
	hostEmail := *res.EmailCached

	// 1. Ensure the host principal exists. Gate the apply JMAP call behind a
	//    no-change check (per the per-tick-idempotent-loops rule): only call
	//    it until the projection is cached. Display-name/metadata changes are
	//    pushed by the REST update handler (the mailgroup.apply pattern), so
	//    the steady-state reconcile pass skips apply entirely.
	if res.HostAccountID == "" {
		applyCtx, applyCancel := context.WithTimeout(ctx, 10*time.Second)
		raw, err := r.agent.Call(applyCtx, "sharedresource.apply", map[string]any{
			"email":        hostEmail,
			"display_name": res.DisplayName,
			"kind":         res.Kind,
		})
		applyCancel()
		if err != nil {
			slog.WarnContext(ctx, "shared_resources: apply host", "resource", res.ID, "err", err)
			return
		}
		var ar struct {
			HostAccountID string `json:"host_account_id"`
		}
		_ = json.Unmarshal(raw, &ar)
		if ar.HostAccountID != "" {
			if err := r.sharedResources.UpdateProjection(ctx, res.ID, ar.HostAccountID, res.CollectionID); err != nil {
				slog.WarnContext(ctx, "shared_resources: cache projection", "resource", res.ID, "err", err)
			}
		}
	}

	// 2. Resolve grants → target email → level. A group grantee expands to its
	//    members; a more permissive direct grant wins over a group-derived one.
	grants, err := r.sharedResources.ListGrants(ctx, res.ID)
	if err != nil {
		slog.WarnContext(ctx, "shared_resources: list grants", "resource", res.ID, "err", err)
		return
	}
	shares := make(map[string]string)
	for _, g := range grants {
		switch g.GranteeKind {
		case "mailbox":
			mb, err := r.srMailboxes.FindByID(ctx, g.GranteeID)
			if err != nil {
				continue
			}
			dom, err := r.domains.FindByID(ctx, mb.DomainID)
			if err != nil {
				continue
			}
			shares[mb.LocalPart+"@"+dom.Name] = g.Rights
		case "group":
			emails, err := r.srMailGroups.ListMemberEmails(ctx, g.GranteeID)
			if err != nil {
				continue
			}
			for _, e := range emails {
				if _, exists := shares[e]; !exists {
					shares[e] = g.Rights
				}
			}
		}
	}

	// 3. Push the whole shareWith map (idempotent replace; empty clears it).
	params := map[string]any{"owner_email": hostEmail, "shares": shares}
	if res.Kind == "files" {
		params["folder_name"] = res.DisplayName
	}
	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if _, err := r.agent.Call(callCtx, cmd, params); err != nil {
		slog.WarnContext(ctx, "shared_resources: share_set", "resource", res.ID, "cmd", cmd, "err", err)
	}
}
