// Package reconciler — scheduled registrar-expiry fetch (GH #259).
//
// Populates domains.registrar_expires_at from a WHOIS lookup (the agent's
// domain.whois command, which shells the `whois` binary). Registrar expiry
// changes ~yearly, so a domain is refreshed at most once every
// registrarExpiryStale; each tick processes a small batch with a per-domain
// delay so we never hammer a registrar's WHOIS server. Entirely best-effort:
// a failed/empty/unparseable lookup still stamps registrar_checked_at so the
// domain isn't re-queried every pass, and never blocks anything.
package reconciler

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/agent"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/repository"
)

const (
	registrarExpiryInterval  = 6 * time.Hour       // how often a batch runs
	registrarExpiryStale     = 7 * 24 * time.Hour   // re-check a domain at most this often
	registrarExpiryBatch     = 25                   // domains per tick
	registrarExpiryPerDomain = 2 * time.Second      // delay between WHOIS lookups
	registrarExpiryCallTO    = 25 * time.Second     // per-domain agent call timeout
)

// StartDomainExpiryTicker runs the registrar-expiry fetch loop until ctx is
// cancelled. Nil deps disable it.
func StartDomainExpiryTicker(ctx context.Context, ag agent.AgentInterface, domains repository.DomainRepository, log bwTickerLogger) {
	if ag == nil || domains == nil {
		return
	}
	log.Info("domain-expiry ticker starting", "interval", registrarExpiryInterval.String())

	// First pass shortly after boot so a fresh install populates without
	// waiting hours; the per-domain delay keeps it gentle.
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(90 * time.Second):
			refreshRegistrarExpiry(ctx, ag, domains, log)
		}
	}()

	t := time.NewTicker(registrarExpiryInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			refreshRegistrarExpiry(ctx, ag, domains, log)
		}
	}
}

func refreshRegistrarExpiry(ctx context.Context, ag agent.AgentInterface, domains repository.DomainRepository, log bwTickerLogger) {
	staleBefore := time.Now().UTC().Add(-registrarExpiryStale)
	rows, err := domains.ListForRegistrarRefresh(ctx, staleBefore, registrarExpiryBatch)
	if err != nil {
		log.Warn("domain-expiry: list failed", "error", err)
		return
	}
	for i := range rows {
		select {
		case <-ctx.Done():
			return
		default:
		}
		d := rows[i]
		expiresAt := lookupRegistrarExpiry(ctx, ag, d.Name, log)
		// Always stamp checked_at (even on nil) so we don't re-query every tick.
		if err := domains.SetRegistrarExpiry(ctx, d.ID, expiresAt, time.Now().UTC()); err != nil {
			log.Warn("domain-expiry: store failed", "domain", d.Name, "error", err)
		}
		// Throttle between registrar WHOIS hits.
		if i < len(rows)-1 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(registrarExpiryPerDomain):
			}
		}
	}
	if len(rows) > 0 {
		log.Info("domain-expiry: pass complete", "checked", len(rows))
	}
}

// lookupRegistrarExpiry dispatches domain.whois and parses the expiry date.
// Returns nil on any failure (no whois data, unparseable date, agent error).
func lookupRegistrarExpiry(ctx context.Context, ag agent.AgentInterface, name string, log bwTickerLogger) *time.Time {
	callCtx, cancel := context.WithTimeout(ctx, registrarExpiryCallTO)
	defer cancel()
	raw, err := ag.Call(callCtx, "domain.whois", map[string]any{"domain": name})
	if err != nil {
		return nil
	}
	var v struct {
		ExpiryDate string `json:"expiry_date"`
	}
	if json.Unmarshal(raw, &v) != nil || strings.TrimSpace(v.ExpiryDate) == "" {
		return nil
	}
	return parseWhoisDate(v.ExpiryDate)
}

// whoisDateLayouts covers the formats registrars actually emit (the value is
// the raw whois string, never normalized upstream).
var whoisDateLayouts = []string{
	time.RFC3339,
	time.RFC3339Nano,
	"2006-01-02T15:04:05Z",
	"2006-01-02T15:04:05.000Z",
	"2006-01-02 15:04:05",
	"2006-01-02",
	"2006.01.02",
	"02-Jan-2006",
	"02-Jan-2006 15:04:05 MST",
	"2006/01/02",
	"Mon Jan 02 15:04:05 MST 2006",
	"January 2 2006",
	"02.01.2006",
}

func parseWhoisDate(s string) *time.Time {
	s = strings.TrimSpace(s)
	// Some registrars append "(yyyy-mm-dd)" or trailing notes; take the first
	// whitespace-trimmed token group up to a paren.
	if i := strings.IndexByte(s, '('); i > 0 {
		s = strings.TrimSpace(s[:i])
	}
	for _, layout := range whoisDateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			u := t.UTC()
			return &u
		}
	}
	return nil
}
