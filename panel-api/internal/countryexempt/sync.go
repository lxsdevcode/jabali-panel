package countryexempt

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/agent"
)

// CountryAllowlistName mirrors the agent's const (ADR-0166). The country
// CIDR sets live in their own LAPI AllowList so they never mix with admin
// trust entries.
const CountryAllowlistName = "jabali-country-allowlist"

// commentPrefix tags every allowlist entry this feature creates, so the
// diff can tell "ours, from country CC" apart from anything else.
const commentPrefix = "country:"

// addChunk caps entries per agent call — one cscli invocation per chunk
// (measured ~5k entries / 26s on testserver; larger chunks risk the agent's
// NDJSON size limit and hide progress).
const addChunk = 4000

// removeChunk caps removals per agent call (agent runs one cscli per value).
const removeChunk = 500

// Syncer computes and applies the CIDR diff for the country exemption.
type Syncer struct {
	Agent    agent.AgentInterface
	HTTP     *http.Client
	CacheDir string
	Log      *slog.Logger
}

// NewSyncer with production defaults.
func NewSyncer(cli agent.AgentInterface, log *slog.Logger) *Syncer {
	if log == nil {
		log = slog.Default()
	}
	return &Syncer{
		Agent:    cli,
		HTTP:     &http.Client{Timeout: 60 * time.Second},
		CacheDir: DefaultCacheDir,
		Log:      log,
	}
}

// csSyncEntry mirrors the agent's csCountryAllowlistSyncEntry wire shape.
type csSyncEntry struct {
	Value   string `json:"value"`
	Comment string `json:"comment"`
}

// syncLocked computes desired-vs-current and applies the delta. Callers
// must serialize — package-level KickBackground is the only entry point
// (PUT kick, CLI sync, weekly refresher all go through it).
func (s *Syncer) syncLocked(ctx context.Context, countries []string, forceRefresh bool) error {
	zc := zoneCache{dir: s.CacheDir}

	// Desired state: cidr → country code.
	desired := map[string]string{}
	for _, code := range countries {
		cidrs, stale, err := loadCountry(ctx, s.HTTP, zc, code, forceRefresh)
		if err != nil {
			return fmt.Errorf("load country %s: %w", code, err)
		}
		if stale {
			s.Log.Warn("countryexempt: using fallback CIDR snapshot", "country", code)
		}
		for _, c := range cidrs {
			desired[c] = code
		}
	}

	// Current state: LAPI is truth (ADR-0061).
	current, err := s.listCurrent(ctx)
	if err != nil {
		return err
	}

	var adds []csSyncEntry
	for cidr, code := range desired {
		if _, ok := current[cidr]; !ok {
			adds = append(adds, csSyncEntry{Value: cidr, Comment: commentPrefix + code})
		}
	}
	var removes []string
	for value, reason := range current {
		// Only manage entries we created. Anything not tagged country:<CC>
		// (an operator's manual add) is left alone.
		if !strings.HasPrefix(reason, commentPrefix) {
			continue
		}
		if _, ok := desired[value]; !ok {
			removes = append(removes, value)
		}
	}

	if len(adds) == 0 && len(removes) == 0 {
		s.Log.Info("countryexempt: already converged", "desired", len(desired))
		return nil
	}

	// Removes first: a range that moved between countries must not exist
	// twice (cscli rejects duplicate values, which our agent tolerates as
	// "already", but removing first keeps the comment truthful).
	for i := 0; i < len(removes); i += removeChunk {
		end := min(i+removeChunk, len(removes))
		if err := s.callSync(ctx, nil, removes[i:end]); err != nil {
			return fmt.Errorf("apply removes: %w", err)
		}
	}
	for i := 0; i < len(adds); i += addChunk {
		end := min(i+addChunk, len(adds))
		if err := s.callSync(ctx, adds[i:end], nil); err != nil {
			return fmt.Errorf("apply adds: %w", err)
		}
		s.Log.Info("countryexempt: applied add chunk", "done", end, "total", len(adds))
	}
	s.Log.Info("countryexempt: sync complete", "added", len(adds), "removed", len(removes), "entries", len(desired))
	return nil
}

// allowlistItem mirrors the agent's csAllowlistEntryWire wire shape.
type allowlistItem struct {
	Value  string `json:"value"`
	Reason string `json:"reason"`
}

// RunSync performs a full sync synchronously in the caller's process —
// used by the `jabali crowdsec country-exempt sync` CLI, which cannot use
// KickBackground: a CLI exits, killing the background goroutine before it
// does any work (observed on testserver 2026-08-13).
func (s *Syncer) RunSync(ctx context.Context, countries []string, forceRefresh bool) error {
	return s.syncLocked(ctx, countries, forceRefresh)
}

// listCurrent returns value → comment for every entry in the country
// allowlist. A missing allowlist reads as empty (first run).
func (s *Syncer) listCurrent(ctx context.Context) (map[string]string, error) {
	raw, err := s.Agent.Call(ctx, "security.crowdsec.allowlists.list", map[string]any{
		"name": CountryAllowlistName,
	})
	if err != nil {
		return nil, fmt.Errorf("list country allowlist: %w", err)
	}
	var payload struct {
		Items []allowlistItem `json:"items"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("parse country allowlist: %w", err)
	}
	out := make(map[string]string, len(payload.Items))
	for _, it := range payload.Items {
		out[it.Value] = it.Reason
	}
	return out, nil
}

func (s *Syncer) callSync(ctx context.Context, adds []csSyncEntry, removes []string) error {
	_, err := s.Agent.Call(ctx, "security.crowdsec.country_allowlist.sync", map[string]any{
		"adds":    adds,
		"removes": removes,
	})
	return err
}
