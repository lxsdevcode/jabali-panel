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
	if err := zc.write("IL", []string{"1.0.0.0/8", "2.0.0.0/8"}); err != nil {
		t.Fatal(err)
	}
	got, mtime, err := zc.read("IL")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "1.0.0.0/8" {
		t.Fatalf("got %v", got)
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

// fakeAgent records Call verbs/params and returns canned responses.
type fakeAgent struct {
	t        *testing.T
	calls    []fakeCall
	listJSON any
}

type fakeCall struct {
	verb   string
	params map[string]any
}

func (f *fakeAgent) Call(_ context.Context, verb string, params any) (json.RawMessage, error) {
	f.calls = append(f.calls, fakeCall{verb: verb, params: params.(map[string]any)})
	if verb == "security.crowdsec.allowlists.list" {
		return json.Marshal(f.listJSON)
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

	if err := s.syncLocked(context.Background(), []string{"IL"}, false); err != nil {
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

	if err := s.syncLocked(context.Background(), nil, false); err != nil {
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

	if err := s.syncLocked(context.Background(), []string{"IL"}, false); err != nil {
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
