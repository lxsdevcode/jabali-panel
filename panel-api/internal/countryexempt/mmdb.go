package countryexempt

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"strings"
)

// mmdbDeriveResponse mirrors the agent's
// security.crowdsec.country_zones.derive payload.
type mmdbDeriveResponse struct {
	Path  string              `json:"path"`
	MTime int64               `json:"mtime"`
	Zones map[string][]string `json:"zones"`
}

// deriveViaAgent asks the agent (root) to walk the local MaxMind database —
// the same file the GeoIP enricher classifies with — and return per-country
// CIDR sets. One call covers every needed country: a full-tree walk costs
// seconds, so per-country calls would multiply that.
func (s *Syncer) deriveViaAgent(ctx context.Context, countries []string) (*mmdbDeriveResponse, error) {
	raw, err := s.Agent.Call(ctx, "security.crowdsec.country_zones.derive", map[string]any{
		"countries": countries,
	})
	if err != nil {
		return nil, fmt.Errorf("derive country zones via agent: %w", err)
	}
	var payload mmdbDeriveResponse
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("parse derive response: %w", err)
	}
	return &payload, nil
}

// mmdbStat returns the mtime (unix) of the mmdb the classifier uses. An
// error means "no info" — callers degrade to age-based snapshot freshness.
func (s *Syncer) mmdbStat(ctx context.Context) (int64, error) {
	raw, err := s.Agent.Call(ctx, "security.crowdsec.mmdb.stat", map[string]any{})
	if err != nil {
		return 0, err
	}
	var payload struct {
		MTime int64 `json:"mtime"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return 0, fmt.Errorf("parse mmdb stat: %w", err)
	}
	return payload.MTime, nil
}

// NormalizeCIDR validates one operator-supplied allowlist entry. Prefixes
// are masked to canonical form; a bare IP becomes a host prefix (/32, /128)
// so every stored entry is a valid cscli allowlist value.
func NormalizeCIDR(s string) (string, error) {
	v := strings.TrimSpace(s)
	if v == "" {
		return "", fmt.Errorf("empty value")
	}
	if p, err := netip.ParsePrefix(v); err == nil {
		return p.Masked().String(), nil
	}
	if a, err := netip.ParseAddr(v); err == nil {
		bits := 32
		if a.Is6() {
			bits = 128
		}
		return netip.PrefixFrom(a, bits).String(), nil
	}
	return "", fmt.Errorf("%q is not an IP or CIDR", s)
}
