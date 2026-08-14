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

// extraTag is the pseudo-country used for the operator's supplemental
// CIDRs (server_settings.country_exempt_extra_cidrs). Managed like a
// country entry: removing it from settings removes it from the allowlist.
const extraTag = "extra"

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
// (PUT kick, CLI sync, weekly refresher all go through it). extras are the
// operator's supplemental CIDRs (already normalized upstream; re-validated
// here defensively).
func (s *Syncer) syncLocked(ctx context.Context, countries []string, extras []string, forceRefresh bool) error {
	zc := zoneCache{dir: s.CacheDir}

	// Desired state: cidr → country code (or extraTag for supplemental
	// entries).
	zones, err := s.loadAll(ctx, zc, countries, forceRefresh)
	if err != nil {
		return err
	}
	desired := map[string]string{}
	for code, cidrs := range zones {
		for _, c := range cidrs {
			desired[c] = code
		}
	}
	for _, raw := range extras {
		c, cerr := NormalizeCIDR(raw)
		if cerr != nil {
			s.Log.Warn("countryexempt: skipping invalid extra CIDR", "value", raw)
			continue
		}
		desired[c] = extraTag
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

// loadAll resolves the CIDR set for every selected country. Source order
// per country: fresh snapshot → mmdb derive (one agent round-trip for all
// needed codes) → ipdeny fetch → stale snapshot. mmdb-derived snapshots
// additionally expire the moment the classifier's mmdb is newer than the
// snapshot's marker — country coverage must track the DB the enricher
// actually reads, not a weekly calendar.
func (s *Syncer) loadAll(ctx context.Context, zc zoneCache, countries []string, forceRefresh bool) (map[string][]string, error) {
	out := make(map[string][]string, len(countries))
	var need []string
	mmdbSourced := map[string]snapshotMeta{}

	for _, code := range countries {
		cidrs, meta, mtime, rerr := zc.read(code)
		switch {
		case rerr == nil && !forceRefresh && time.Since(mtime) < snapshotMaxAge:
			out[code] = cidrs
			if meta.source == sourceMMDB {
				mmdbSourced[code] = meta
			}
		default:
			need = append(need, code)
		}
	}

	// mmdb freshness check — one cheap stat for all mmdb-sourced snapshots.
	if len(mmdbSourced) > 0 && !forceRefresh {
		if mtime, serr := s.mmdbStat(ctx); serr == nil {
			for code, meta := range mmdbSourced {
				if mtime > meta.mmdbMTime {
					s.Log.Info("countryexempt: mmdb newer than snapshot, re-deriving",
						"country", code, "snapshot_mtime", meta.mmdbMTime, "mmdb_mtime", mtime)
					delete(out, code)
					need = append(need, code)
				}
			}
		} else {
			s.Log.Warn("countryexempt: mmdb stat failed, keeping age-based freshness", "err", serr)
		}
	}

	if len(need) == 0 {
		return out, nil
	}
	derived, derr := s.deriveViaAgent(ctx, need)
	if derr != nil {
		s.Log.Warn("countryexempt: mmdb derive unavailable, falling back to ipdeny", "err", derr)
	}
	for _, code := range need {
		if derr == nil && len(derived.Zones[code]) > 0 {
			merged := mergeCIDRs(derived.Zones[code])
			if werr := zc.write(code, merged, snapshotMeta{source: sourceMMDB, mmdbMTime: derived.MTime}); werr != nil {
				// Non-fatal: proceed with the in-memory set (same policy as
				// the ipdeny path).
				s.Log.Warn("countryexempt: snapshot write failed", "country", code, "err", werr)
			}
			out[code] = merged
			s.Log.Info("countryexempt: derived zone from mmdb", "country", code,
				"raw", len(derived.Zones[code]), "merged", len(merged))
			continue
		}
		// Derive failed or the mmdb has no networks for this code — ipdeny
		// (which itself falls back to any stale snapshot on fetch failure).
		cidrs, stale, ferr := loadCountry(ctx, s.HTTP, zc, code, true)
		if ferr != nil {
			return nil, fmt.Errorf("load country %s: %w", code, ferr)
		}
		if stale {
			s.Log.Warn("countryexempt: using fallback CIDR snapshot", "country", code)
		}
		out[code] = cidrs
	}
	return out, nil
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
func (s *Syncer) RunSync(ctx context.Context, countries []string, extras []string, forceRefresh bool) error {
	return s.syncLocked(ctx, countries, extras, forceRefresh)
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
