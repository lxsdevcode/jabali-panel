package commands

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"testing"
)

type fakeMMDBEntry struct {
	network string
	rec     mmdbISORecord
}

// fakeMMDBIter plays back a fixed set of (network, record) pairs, then stops
// (or fails) — enough to exercise deriveZones without a writer-side mmdb.
type fakeMMDBIter struct {
	entries []fakeMMDBEntry
	idx     int
	err     error
}

func (f *fakeMMDBIter) Next() bool { return f.err == nil && f.idx < len(f.entries) }

func (f *fakeMMDBIter) Network(result any) (*net.IPNet, error) {
	e := f.entries[f.idx]
	f.idx++
	rec, ok := result.(*mmdbISORecord)
	if !ok {
		return nil, errors.New("unexpected result type")
	}
	*rec = e.rec
	_, n, err := net.ParseCIDR(e.network)
	if err != nil {
		return nil, err
	}
	return n, nil
}

func (f *fakeMMDBIter) Err() error { return f.err }

func isoRec(country, registered, represented string) mmdbISORecord {
	var r mmdbISORecord
	r.Country.ISOCode = country
	r.RegisteredCountry.ISOCode = registered
	r.RepresentedCountry.ISOCode = represented
	return r
}

func TestDeriveZones_FiltersAndBuckets(t *testing.T) {
	iter := &fakeMMDBIter{entries: []fakeMMDBEntry{
		{"1.0.0.0/24", isoRec("IL", "", "")},
		{"2.0.0.0/24", isoRec("US", "", "")},
		{"2001:db8::/32", isoRec("IL", "", "")},
		{"3.0.0.0/24", isoRec("", "", "")}, // no record country — skipped
	}}
	zones, err := deriveZones(iter, map[string]bool{"IL": true})
	if err != nil {
		t.Fatalf("deriveZones: %v", err)
	}
	if len(zones) != 1 {
		t.Fatalf("expected only IL bucket, got %v", zones)
	}
	got := zones["IL"]
	want := []string{"1.0.0.0/24", "2001:db8::/32"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("IL zone = %v, want %v", got, want)
	}
}

// The precedence must match crowdsec's GeoIpCity bit-for-bit:
// Country → RegisteredCountry → RepresentedCountry.
func TestDeriveZones_ISOCodePrecedence(t *testing.T) {
	iter := &fakeMMDBIter{entries: []fakeMMDBEntry{
		{"1.0.0.0/24", isoRec("IL", "US", "DE")},   // Country wins
		{"2.0.0.0/24", isoRec("", "US", "DE")},     // Registered next
		{"3.0.0.0/24", isoRec("", "", "DE")},       // Represented last
		{"4.0.0.0/24", isoRec("", "gb", "")},       // NOT uppercased by mmdb — kept verbatim
	}}
	zones, err := deriveZones(iter, map[string]bool{"IL": true, "US": true, "DE": true})
	if err != nil {
		t.Fatalf("deriveZones: %v", err)
	}
	if got := zones["IL"]; len(got) != 1 || got[0] != "1.0.0.0/24" {
		t.Fatalf("IL = %v", got)
	}
	if got := zones["US"]; len(got) != 1 || got[0] != "2.0.0.0/24" {
		t.Fatalf("US = %v", got)
	}
	if got := zones["DE"]; len(got) != 1 || got[0] != "3.0.0.0/24" {
		t.Fatalf("DE = %v", got)
	}
}

func TestDeriveZones_IteratorError(t *testing.T) {
	iter := &fakeMMDBIter{err: errors.New("corrupt tree")}
	if _, err := deriveZones(iter, map[string]bool{"IL": true}); err == nil {
		t.Fatal("expected error from failing iterator")
	}
}

func TestCsCountryZonesDeriveHandler_RejectsBadInput(t *testing.T) {
	for _, params := range []string{
		`{"countries": []}`,
		`{"countries": ["USA"]}`,
		`{"countries": ["i1"]}`,
		`not-json`,
	} {
		if _, err := csCountryZonesDeriveHandler(context.Background(), json.RawMessage(params)); err == nil {
			t.Fatalf("params %s: expected error", params)
		}
	}
}

func TestCsMMDBStatHandler_NoMMDBIsError(t *testing.T) {
	// Dev machines and CI runners have no /var/lib/crowdsec/data — the verb
	// must surface a clean error, not panic.
	if _, err := csMMDBStatHandler(context.Background(), nil); err == nil {
		t.Skip("host has a crowdsec mmdb — error path not exercised")
	}
}
