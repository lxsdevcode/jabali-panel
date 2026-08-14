package countryexempt

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseZone(t *testing.T) {
	body := "# comment\n1.2.3.0/24\n\n2001:db8::/32\nnot-a-cidr\n  5.6.7.0/24  \n"
	got := parseZone(body)
	want := []string{"1.2.3.0/24", "2001:db8::/32", "5.6.7.0/24"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestZoneCacheRoundTrip(t *testing.T) {
	zc := zoneCache{dir: t.TempDir()}
	if err := zc.write("IL", []string{"1.0.0.0/8", "2.0.0.0/8"}, snapshotMeta{source: sourceMMDB, mmdbMTime: 1234}); err != nil {
		t.Fatal(err)
	}
	got, meta, mtime, err := zc.read("IL")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "1.0.0.0/8" {
		t.Fatalf("got %v", got)
	}
	if meta.source != sourceMMDB || meta.mmdbMTime != 1234 {
		t.Fatalf("meta = %+v", meta)
	}
	if time.Since(mtime) > time.Minute {
		t.Fatalf("mtime should be fresh: %v", mtime)
	}
}

// zoneTestServer serves ipdeny-shaped zone files for both URL templates.
func zoneTestServer(t *testing.T, v4, v6 string, fail bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		if strings.Contains(r.URL.Path, "ipv6") {
			fmt.Fprint(w, v6)
		} else {
			fmt.Fprint(w, v4)
		}
	}))
}

func withZoneURLs(t *testing.T, srv *httptest.Server) {
	t.Helper()
	orig := zoneURLTemplates
	zoneURLTemplates = []string{
		srv.URL + "/ipblocks/data/aggregated/%s-aggregated.zone",
		srv.URL + "/ipv6/ipaddresses/aggregated/%s-aggregated.zone",
	}
	t.Cleanup(func() { zoneURLTemplates = orig })
}

func TestLoadCountry_FetchesV4AndV6(t *testing.T) {
	srv := zoneTestServer(t, "1.0.0.0/8\n", "2001:db8::/32\n", false)
	withZoneURLs(t, srv)
	zc := zoneCache{dir: t.TempDir()}

	got, stale, err := loadCountry(context.Background(), srv.Client(), zc, "il", false)
	if err != nil {
		t.Fatal(err)
	}
	if stale {
		t.Fatal("fresh fetch must not be marked stale")
	}
	if len(got) != 2 {
		t.Fatalf("expected v4+v6 merged, got %v", got)
	}
	// Second call must hit the snapshot, not the network (mtime fresh).
	got2, stale2, err := loadCountry(context.Background(), srv.Client(), zc, "IL", false)
	if err != nil || stale2 || len(got2) != 2 {
		t.Fatalf("snapshot read: got %v stale %v err %v", got2, stale2, err)
	}
}

func TestLoadCountry_FetchFailureFallsBackToSnapshot(t *testing.T) {
	good := zoneTestServer(t, "1.0.0.0/8\n", "2001:db8::/32\n", false)
	withZoneURLs(t, good)
	zc := zoneCache{dir: t.TempDir()}
	if _, _, err := loadCountry(context.Background(), good.Client(), zc, "IL", true); err != nil {
		t.Fatal(err)
	}

	// Point at a failing server; force refresh. Must fall back to snapshot.
	bad := zoneTestServer(t, "", "", true)
	withZoneURLs(t, bad)
	got, stale, err := loadCountry(context.Background(), bad.Client(), zc, "IL", true)
	if err != nil {
		t.Fatal(err)
	}
	if !stale {
		t.Fatal("fallback must be marked stale")
	}
	if len(got) != 2 {
		t.Fatalf("expected snapshot data, got %v", got)
	}
}

func TestLoadCountry_NoSnapshotNoNetwork_IsError(t *testing.T) {
	bad := zoneTestServer(t, "", "", true)
	withZoneURLs(t, bad)
	zc := zoneCache{dir: t.TempDir()}
	if _, _, err := loadCountry(context.Background(), bad.Client(), zc, "IL", true); err == nil {
		t.Fatal("expected error with no snapshot and no network")
	}
}

// fakeAgent records Call verbs/params and returns canned responses. The
// derive verb returns deriveErr / deriveZones so tests can pick between the
// mmdb path and the ipdeny fallback; stat returns statErr / statMTime.
type fakeAgent struct {
	t          *testing.T
	calls      []fakeCall
	listJSON   any
	deriveErr  error
	deriveZone map[string][]string
	deriveMT   int64
	statErr    error
	statMTime  int64
}

type fakeCall struct {
	verb   string
	params map[string]any
}

func (f *fakeAgent) Call(_ context.Context, verb string, params any) (json.RawMessage, error) {
	p, _ := params.(map[string]any)
	f.calls = append(f.calls, fakeCall{verb: verb, params: p})
	switch verb {
	case "security.crowdsec.allowlists.list":
		return json.Marshal(f.listJSON)
	case "security.crowdsec.country_zones.derive":
		if f.deriveErr != nil {
			return nil, f.deriveErr
		}
		return json.Marshal(mmdbDeriveResponse{Path: "/fake.mmdb", MTime: f.deriveMT, Zones: f.deriveZone})
	case "security.crowdsec.mmdb.stat":
		if f.statErr != nil {
			return nil, f.statErr
		}
		return json.Marshal(map[string]any{"path": "/fake.mmdb", "mtime": f.statMTime, "size": 1})
	}
	return json.RawMessage(`{}`), nil
}

func newTestSyncer(t *testing.T, srv *httptest.Server, listJSON any) (*Syncer, *fakeAgent) {
	t.Helper()
	fa := &fakeAgent{t: t, listJSON: listJSON}
	withZoneURLs(t, srv)
	return &Syncer{
		Agent:    fa,
		HTTP:     srv.Client(),
		CacheDir: t.TempDir(),
		Log:      slog.Default(),
	}, fa
}

func TestSync_FullFlow_AddsOnlyNew(t *testing.T) {
	srv := zoneTestServer(t, "1.0.0.0/8\n2.0.0.0/8\n", "2001:db8::/32\n", false)
	// 2.0.0.0/8 already present with the right comment; 9.9.0.0/16 is a
	// stale entry of ours; "office LAN" is a manual entry we must keep.
	listJSON := map[string]any{"items": []map[string]string{
		{"value": "2.0.0.0/8", "reason": "country:IL"},
		{"value": "9.9.0.0/16", "reason": "country:IL"},
		{"value": "10.0.0.0/8", "reason": "office LAN"},
	}}
	s, fa := newTestSyncer(t, srv, listJSON)

	if err := s.syncLocked(context.Background(), []string{"IL"}, nil, false); err != nil {
		t.Fatal(err)
	}

	var addedEntries []csSyncEntry
	var removedValues []string
	for _, c := range fa.calls {
		if c.verb != "security.crowdsec.country_allowlist.sync" {
			continue
		}
		addedEntries = append(addedEntries, c.params["adds"].([]csSyncEntry)...)
		removedValues = append(removedValues, c.params["removes"].([]string)...)
	}
	if len(removedValues) != 1 || removedValues[0] != "9.9.0.0/16" {
		t.Fatalf("removes = %v, want [9.9.0.0/16]", removedValues)
	}
	var addedValues []string
	for _, e := range addedEntries {
		addedValues = append(addedValues, e.Value)
		if e.Comment != "country:IL" {
			t.Fatalf("comment = %q, want country:IL", e.Comment)
		}
	}
	if len(addedValues) != 2 {
		t.Fatalf("adds = %v, want 1.0.0.0/8 + 2001:db8::/32", addedValues)
	}
	for _, v := range addedValues {
		if v == "10.0.0.0/8" || v == "2.0.0.0/8" || v == "9.9.0.0/16" {
			t.Fatalf("unexpected add %v", v)
		}
	}
}

func TestSync_EmptySelection_RemovesOnlyOurs(t *testing.T) {
	srv := zoneTestServer(t, "", "", false)
	listJSON := map[string]any{"items": []map[string]string{
		{"value": "1.0.0.0/8", "reason": "country:IL"},
		{"value": "10.0.0.0/8", "reason": "office LAN"},
	}}
	s, fa := newTestSyncer(t, srv, listJSON)

	if err := s.syncLocked(context.Background(), nil, nil, false); err != nil {
		t.Fatal(err)
	}
	var removedValues []string
	for _, c := range fa.calls {
		if c.verb == "security.crowdsec.country_allowlist.sync" {
			removedValues = append(removedValues, c.params["removes"].([]string)...)
		}
	}
	if len(removedValues) != 1 || removedValues[0] != "1.0.0.0/8" {
		t.Fatalf("removes = %v — manual entries must be left alone", removedValues)
	}
}

func TestSync_Converged_NoAgentSyncCalls(t *testing.T) {
	srv := zoneTestServer(t, "1.0.0.0/8\n", "2001:db8::/32\n", false)
	listJSON := map[string]any{"items": []map[string]string{
		{"value": "1.0.0.0/8", "reason": "country:IL"},
		{"value": "2001:db8::/32", "reason": "country:IL"},
	}}
	s, fa := newTestSyncer(t, srv, listJSON)

	if err := s.syncLocked(context.Background(), []string{"IL"}, nil, false); err != nil {
		t.Fatal(err)
	}
	for _, c := range fa.calls {
		if c.verb == "security.crowdsec.country_allowlist.sync" {
			t.Fatal("converged state must not call sync")
		}
	}
}

func TestSplitCountries(t *testing.T) {
	if got := SplitCountries(""); got != nil {
		t.Fatalf("empty csv: %v", got)
	}
	got := SplitCountries("IL, US ,,")
	if len(got) != 2 || got[0] != "IL" || got[1] != "US" {
		t.Fatalf("got %v", got)
	}
}

// ---- mmdb-backed derivation (ADR-0166 amendment) ----------------------------

func TestLoadAll_PrefersMMDBOverIPDeny(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		fmt.Fprint(w, "9.9.9.0/24\n")
	}))
	withZoneURLs(t, srv)
	fa := &fakeAgent{t: t, listJSON: map[string]any{"items": []any{}},
		deriveZone: map[string][]string{"IL": {"1.0.0.0/25", "1.0.0.128/25"}},
		deriveMT:   42}
	s := &Syncer{Agent: fa, HTTP: srv.Client(), CacheDir: t.TempDir(), Log: slog.Default()}

	if err := s.syncLocked(context.Background(), []string{"IL"}, nil, false); err != nil {
		t.Fatal(err)
	}
	if hits != 0 {
		t.Fatalf("ipdeny must not be consulted when derive succeeds (hits=%d)", hits)
	}
	// Siblings must arrive merged, tagged with the country.
	var adds []csSyncEntry
	for _, c := range fa.calls {
		if c.verb == "security.crowdsec.country_allowlist.sync" {
			adds = append(adds, c.params["adds"].([]csSyncEntry)...)
		}
	}
	if len(adds) != 1 || adds[0].Value != "1.0.0.0/24" || adds[0].Comment != "country:IL" {
		t.Fatalf("adds = %+v, want merged 1.0.0.0/24 country:IL", adds)
	}
	// Snapshot must be mmdb-sourced with the marker.
	_, meta, _, err := zoneCache{dir: s.CacheDir}.read("IL")
	if err != nil {
		t.Fatal(err)
	}
	if meta.source != sourceMMDB || meta.mmdbMTime != 42 {
		t.Fatalf("snapshot meta = %+v", meta)
	}
}

func TestLoadAll_DeriveFailure_FallsBackToIPDeny(t *testing.T) {
	srv := zoneTestServer(t, "1.0.0.0/8\n", "", false)
	fa := &fakeAgent{t: t, listJSON: map[string]any{"items": []any{}},
		deriveErr: fmt.Errorf("no mmdb")}
	s := &Syncer{Agent: fa, HTTP: srv.Client(), CacheDir: t.TempDir(), Log: slog.Default()}
	withZoneURLs(t, srv)

	if err := s.syncLocked(context.Background(), []string{"IL"}, nil, false); err != nil {
		t.Fatal(err)
	}
	var adds []csSyncEntry
	for _, c := range fa.calls {
		if c.verb == "security.crowdsec.country_allowlist.sync" {
			adds = append(adds, c.params["adds"].([]csSyncEntry)...)
		}
	}
	if len(adds) != 1 || adds[0].Value != "1.0.0.0/8" {
		t.Fatalf("adds = %+v, want ipdeny 1.0.0.0/8", adds)
	}
}

func TestLoadAll_MMDBNewerThanSnapshot_ReDerives(t *testing.T) {
	zc := zoneCache{dir: t.TempDir()}
	if err := zc.write("IL", []string{"1.0.0.0/8"}, snapshotMeta{source: sourceMMDB, mmdbMTime: 100}); err != nil {
		t.Fatal(err)
	}
	srv := zoneTestServer(t, "", "", false)
	fa := &fakeAgent{t: t, listJSON: map[string]any{"items": []any{}},
		statMTime:  200, // classifier DB updated after the snapshot
		deriveZone: map[string][]string{"IL": {"2.0.0.0/8"}},
		deriveMT:   200}
	s := &Syncer{Agent: fa, HTTP: srv.Client(), CacheDir: zc.dir, Log: slog.Default()}
	withZoneURLs(t, srv)

	if err := s.syncLocked(context.Background(), []string{"IL"}, nil, false); err != nil {
		t.Fatal(err)
	}
	derived := false
	for _, c := range fa.calls {
		if c.verb == "security.crowdsec.country_zones.derive" {
			derived = true
		}
	}
	if !derived {
		t.Fatal("mmdb newer than snapshot must trigger re-derivation")
	}
	var adds []csSyncEntry
	for _, c := range fa.calls {
		if c.verb == "security.crowdsec.country_allowlist.sync" {
			adds = append(adds, c.params["adds"].([]csSyncEntry)...)
		}
	}
	if len(adds) != 1 || adds[0].Value != "2.0.0.0/8" {
		t.Fatalf("adds = %+v, want re-derived 2.0.0.0/8", adds)
	}
}

func TestLoadAll_MMDBUnchanged_KeepsSnapshot(t *testing.T) {
	zc := zoneCache{dir: t.TempDir()}
	if err := zc.write("IL", []string{"1.0.0.0/8"}, snapshotMeta{source: sourceMMDB, mmdbMTime: 100}); err != nil {
		t.Fatal(err)
	}
	srv := zoneTestServer(t, "", "", false)
	fa := &fakeAgent{t: t, listJSON: map[string]any{"items": []any{
		map[string]string{"value": "1.0.0.0/8", "reason": "country:IL"},
	}}, statMTime: 100} // same mtime — snapshot still valid
	s := &Syncer{Agent: fa, HTTP: srv.Client(), CacheDir: zc.dir, Log: slog.Default()}
	withZoneURLs(t, srv)

	if err := s.syncLocked(context.Background(), []string{"IL"}, nil, false); err != nil {
		t.Fatal(err)
	}
	for _, c := range fa.calls {
		if c.verb == "security.crowdsec.country_zones.derive" {
			t.Fatal("unchanged mmdb must not re-derive")
		}
		if c.verb == "security.crowdsec.country_allowlist.sync" {
			t.Fatal("converged state must not sync")
		}
	}
}

func TestSync_ExtraCIDRs_ManagedAsCountryExtra(t *testing.T) {
	srv := zoneTestServer(t, "1.0.0.0/8\n", "", false)
	listJSON := map[string]any{"items": []map[string]string{
		{"value": "8.8.8.8/32", "reason": "country:extra"}, // stale extra — must go
		{"value": "10.0.0.0/8", "reason": "office LAN"},    // manual — must stay
	}}
	s, fa := newTestSyncer(t, srv, listJSON)

	err := s.syncLocked(context.Background(), []string{"IL"}, []string{"203.0.113.7", "192.0.2.0/25"}, false)
	if err != nil {
		t.Fatal(err)
	}
	var adds []csSyncEntry
	var removes []string
	for _, c := range fa.calls {
		if c.verb != "security.crowdsec.country_allowlist.sync" {
			continue
		}
		adds = append(adds, c.params["adds"].([]csSyncEntry)...)
		removes = append(removes, c.params["removes"].([]string)...)
	}
	var extraAdds []string
	for _, a := range adds {
		if a.Comment == "country:extra" {
			extraAdds = append(extraAdds, a.Value)
		}
	}
	if len(extraAdds) != 2 {
		t.Fatalf("extra adds = %v, want 203.0.113.7/32 + 192.0.2.0/25", extraAdds)
	}
	if len(removes) != 1 || removes[0] != "8.8.8.8/32" {
		t.Fatalf("removes = %v, want [8.8.8.8/32]", removes)
	}
}

func TestSync_InvalidExtraCIDR_Skipped(t *testing.T) {
	srv := zoneTestServer(t, "1.0.0.0/8\n", "", false)
	s, fa := newTestSyncer(t, srv, map[string]any{"items": []any{}})

	err := s.syncLocked(context.Background(), []string{"IL"}, []string{"garbage"}, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range fa.calls {
		if c.verb == "security.crowdsec.country_allowlist.sync" {
			for _, a := range c.params["adds"].([]csSyncEntry) {
				if a.Comment == "country:extra" {
					t.Fatalf("invalid extra must not be synced: %+v", a)
				}
			}
		}
	}
}
