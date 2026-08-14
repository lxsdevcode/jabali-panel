package commands

// MaxMind-mmdb-backed country zone derivation for the country ban exemption
// (ADR-0166 amendment, 2026-08-14). The GeoIP enricher classifies IPs with a
// MaxMind database; seeding the LAPI country allowlist from ipdeny zone files
// left gaps wherever MaxMind and registry data disagree (observed:
// 182.54.236.64 is IL per MaxMind but absent from ipdeny's IL zone). Deriving
// the CIDR set from the same mmdb the classifier reads makes allowlist
// coverage exact by construction. ipdeny stays as panel-api's fallback when
// no mmdb is present.
//
// The mmdb is 0600 root under /var/lib/crowdsec/data and panel-api runs as
// the unprivileged jabali user (systemd ProtectSystem=strict), so the agent
// — root — does the iteration and returns plain CIDR strings over the wire.

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"

	"github.com/oschwald/maxminddb-golang"
)

// crowdsecMMDBCandidates in preference order. GeoLite2-City is what
// crowdsecurity/geoip-enrich fetches (dest_file GeoLite2-City.mmdb), so it
// is the database whose IsoCode opinion the scenarios actually see. The
// Country DB is accepted as a degraded substitute when a host only has it.
var crowdsecMMDBCandidates = []string{
	"/var/lib/crowdsec/data/GeoLite2-City.mmdb",
	"/var/lib/crowdsec/data/GeoLite2-Country.mmdb",
}

// crowdsecMMDBPath returns the first readable candidate mmdb.
func crowdsecMMDBPath() (string, error) {
	for _, p := range crowdsecMMDBCandidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("no MaxMind database found under /var/lib/crowdsec/data " +
		"(geoip-enrich installs GeoLite2-City.mmdb — is the GeoIP enricher installed?)")
}

// mmdbISORecord decodes only the ISO codes — maxminddb decodes requested
// fields only, keeping a full-tree walk cheap.
type mmdbISORecord struct {
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
	RegisteredCountry struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"registered_country"`
	RepresentedCountry struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"represented_country"`
}

// isoCode mirrors crowdsec's GeoIpCity precedence EXACTLY
// (pkg/parser/enrich_geoip.go): Country → RegisteredCountry →
// RepresentedCountry → "". Diverging here would re-open the coverage gap
// this verb exists to close.
func (r mmdbISORecord) isoCode() string {
	switch {
	case r.Country.ISOCode != "":
		return r.Country.ISOCode
	case r.RegisteredCountry.ISOCode != "":
		return r.RegisteredCountry.ISOCode
	case r.RepresentedCountry.ISOCode != "":
		return r.RepresentedCountry.ISOCode
	}
	return ""
}

// mmdbNetworks is the slice of *maxminddb.Networks the deriver needs. An
// interface so tests can feed a fake tree without a writer-side mmdb.
type mmdbNetworks interface {
	Next() bool
	Network(result any) (*net.IPNet, error)
	Err() error
}

// deriveZones walks every data-bearing network and buckets it by ISO code.
// Coverage is exact: an IP the classifier resolves to a record for country
// CC always falls inside the data-bearing network that yielded that record
// (lookups return the nearest data-bearing ancestor), and that network is
// in zones[CC]. Merging adjacent CIDRs happens panel-side — the agent
// stays dumb (ADR-0050).
func deriveZones(iter mmdbNetworks, want map[string]bool) (map[string][]string, error) {
	zones := map[string][]string{}
	for iter.Next() {
		var rec mmdbISORecord
		network, err := iter.Network(&rec)
		if err != nil {
			return nil, fmt.Errorf("decode mmdb record: %w", err)
		}
		iso := rec.isoCode()
		if iso == "" || !want[iso] {
			continue
		}
		zones[iso] = append(zones[iso], network.String())
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("walk mmdb: %w", err)
	}
	return zones, nil
}

type csCountryZonesDeriveParams struct {
	Countries []string `json:"countries"`
}

// csCountryZonesDeriveHandler returns the per-country CIDR sets derived
// from the local MaxMind database. Countries with no data-bearing network
// come back with an empty list (not an error) so panel-api can fall back
// to ipdeny for just those codes.
func csCountryZonesDeriveHandler(_ context.Context, params json.RawMessage) (any, error) {
	var p csCountryZonesDeriveParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, csInvalidArg(fmt.Sprintf("parse params: %v", err))
	}
	cleaned, cerr := csCleanCountries(p.Countries)
	if cerr != nil {
		return nil, csInvalidArg(cerr.Error())
	}
	if len(cleaned) == 0 {
		return nil, csInvalidArg("countries required")
	}
	path, err := crowdsecMMDBPath()
	if err != nil {
		return nil, csInternal("locate mmdb", err)
	}
	// #nosec G304 — path comes from the fixed candidate list above.
	reader, err := maxminddb.Open(path)
	if err != nil {
		return nil, csInternal("open mmdb", err)
	}
	defer func() { _ = reader.Close() }()

	want := make(map[string]bool, len(cleaned))
	for _, c := range cleaned {
		want[c] = true
	}
	zones, err := deriveZones(reader.Networks(maxminddb.SkipAliasedNetworks), want)
	if err != nil {
		return nil, csInternal("derive zones", err)
	}
	// Ensure every requested key is present (empty slice, never missing) so
	// panel-api can distinguish "no networks" from a truncated response.
	for _, c := range cleaned {
		if _, ok := zones[c]; !ok {
			zones[c] = []string{}
		}
	}
	st, err := os.Stat(path)
	if err != nil {
		return nil, csInternal("stat mmdb", err)
	}
	return map[string]any{
		"path":  path,
		"mtime": st.ModTime().Unix(),
		"zones": zones,
	}, nil
}

// csMMDBStatHandler reports which mmdb the classifier uses and when it last
// changed. panel-api's refresher compares mtime against its zone snapshots
// so a GeoIP update triggers re-derivation without waiting for the weekly
// staleness window.
func csMMDBStatHandler(_ context.Context, _ json.RawMessage) (any, error) {
	path, err := crowdsecMMDBPath()
	if err != nil {
		return nil, csInternal("locate mmdb", err)
	}
	st, err := os.Stat(path)
	if err != nil {
		return nil, csInternal("stat mmdb", err)
	}
	return map[string]any{
		"path":  path,
		"mtime": st.ModTime().Unix(),
		"size":  st.Size(),
	}, nil
}

// csCleanCountries is defined in security_crowdsec.go (shared with the
// geoblock and country-exemption set handlers).

func init() {
	Default.Register("security.crowdsec.country_zones.derive", csCountryZonesDeriveHandler)
	Default.Register("security.crowdsec.mmdb.stat", csMMDBStatHandler)
}
