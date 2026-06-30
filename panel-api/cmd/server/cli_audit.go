package main

import (
	"context"
	"time"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/ids"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/models"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/repository"
)

// cliAudit writes a unified audit event for a direct root-CLI mutation
// (Gitea #537). Direct CLI commands bypass the HTTP layer (which records via
// the audit middleware), so operator actions were invisible in the trail.
//
// actor_kind="cli", actor_user_id NULL (no logged-in identity on the root
// shell). The hash-chain columns are left NULL — the panel-api single-writer
// chain consumer backfills "fallback-pending" rows (audit_event.go), which is
// the designed path for out-of-process writers. Best-effort + synchronous: a
// short-lived CLI process can't rely on an async recorder goroutine flushing,
// and an audit failure must never fail the actual operation.
//
// target is the resource id/name (NEVER a secret). subjectUserID is the owner
// of the affected resource when known (nil for server-scoped actions).
func cliAudit(ctx context.Context, action, targetType, target, result string, subjectUserID *string) {
	if sharedDB == nil {
		return
	}
	repo := repository.NewAuditEventRepository(sharedDB)
	_ = repo.Create(ctx, &models.AuditEvent{
		ID:            ids.NewULID(),
		TS:            time.Now().UTC(),
		ActorKind:     "cli",
		ActorUserID:   nil,
		SubjectUserID: subjectUserID,
		Action:        action,
		TargetType:    targetType,
		TargetID:      target,
		Result:        result,
	})
}

// cliAuditOK / cliAuditErr are the common-case shorthands.
func cliAuditOK(ctx context.Context, action, targetType, target string, subjectUserID *string) {
	cliAudit(ctx, action, targetType, target, models.AuditResultOK, subjectUserID)
}

func cliAuditErr(ctx context.Context, action, targetType, target string, subjectUserID *string) {
	cliAudit(ctx, action, targetType, target, models.AuditResultError, subjectUserID)
}
