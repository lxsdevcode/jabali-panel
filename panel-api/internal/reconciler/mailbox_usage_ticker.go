// Package reconciler — mailbox usage sampler (GH #263).
//
// Keeps mailboxes.last_usage_bytes fresh for the Mail UI's progress bar. The
// agent's mailbox.usage command (a JMAP read against Stalwart) and the repo's
// UpdateUsage writeback both existed, but nothing ever called them — so Last
// Usage stayed 0 forever. This ticker is the missing sampler: every
// mailboxUsageInterval it reads each mailbox's used_bytes and writes it back.
//
// Best-effort: a failed sample for one mailbox is logged and skipped; a
// brand-new mailbox whose owner hasn't authenticated yet reads 0 (the agent's
// lazy-sync note), which is correct.
package reconciler

import (
	"context"
	"encoding/json"
	"time"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/agent"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/repository"
)

const (
	mailboxUsageInterval = 10 * time.Minute
	mailboxUsagePerMbox  = 150 * time.Millisecond // gentle stagger (JMAP is loopback)
	mailboxUsageCallTO   = 15 * time.Second
)

// StartMailboxUsageTicker runs the usage sampler until ctx is cancelled.
// Nil deps disable it.
func StartMailboxUsageTicker(ctx context.Context, ag agent.AgentInterface, mailboxes repository.MailboxRepository, log bwTickerLogger) {
	if ag == nil || mailboxes == nil {
		return
	}
	log.Info("mailbox-usage ticker starting", "interval", mailboxUsageInterval.String())

	// First pass shortly after boot so the UI populates without waiting.
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(60 * time.Second):
			sampleMailboxUsage(ctx, ag, mailboxes, log)
		}
	}()

	t := time.NewTicker(mailboxUsageInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			sampleMailboxUsage(ctx, ag, mailboxes, log)
		}
	}
}

func sampleMailboxUsage(ctx context.Context, ag agent.AgentInterface, mailboxes repository.MailboxRepository, log bwTickerLogger) {
	rows, err := mailboxes.ListAllWithDomain(ctx)
	if err != nil {
		log.Warn("mailbox-usage: list failed", "error", err)
		return
	}
	sampled := 0
	for i := range rows {
		select {
		case <-ctx.Done():
			return
		default:
		}
		mb := rows[i]
		if used, ok := mailboxUsedBytes(ctx, ag, mb.ID, mb.EmailCached); ok {
			if err := mailboxes.UpdateUsage(ctx, mb.ID, used, time.Now().UTC()); err != nil {
				log.Warn("mailbox-usage: writeback failed", "mailbox", mb.EmailCached, "error", err)
			} else {
				sampled++
			}
		}
		if i < len(rows)-1 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(mailboxUsagePerMbox):
			}
		}
	}
	if len(rows) > 0 {
		log.Info("mailbox-usage: pass complete", "sampled", sampled, "total", len(rows))
	}
}

// mailboxUsedBytes dispatches mailbox.usage and returns used_bytes. ok=false on
// any agent/decode error (logged by the caller's skip), so a single bad
// mailbox never stops the pass.
func mailboxUsedBytes(ctx context.Context, ag agent.AgentInterface, id, email string) (uint64, bool) {
	callCtx, cancel := context.WithTimeout(ctx, mailboxUsageCallTO)
	defer cancel()
	raw, err := ag.Call(callCtx, "mailbox.usage", map[string]any{"id": id, "email": email})
	if err != nil {
		return 0, false
	}
	var v struct {
		UsedBytes uint64 `json:"used_bytes"`
	}
	if json.Unmarshal(raw, &v) != nil {
		return 0, false
	}
	return v.UsedBytes, true
}
