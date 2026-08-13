package commands

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"git.jabali-panel.com/shukivaknin/jabali2/agentwire"
)

// Golden test for the ADR-0166 s02-enrich GeoIP parser whitelist. The
// zz- filename prefix + `in [...]` expr shape are load-bearing: the file
// must sort after crowdsecurity/geoip-enrich within the stage or
// evt.Enriched.IsoCode is empty and the whitelist never matches.
func TestRenderCountryExemptWhitelist_Golden(t *testing.T) {
	got := renderCountryExemptWhitelist([]string{"IL", "US"})
	want := `# Managed by jabali — country ban exemption (ADR-0166).
# Regenerated from DB by security.crowdsec.country_exempt.set.
# Hand edits are overwritten. An empty selection removes this file.
name: jabali/country-allowlist
description: "Never create alerts for IPs from jabali-exempt countries"
whitelist:
  reason: "jabali country exemption"
  expression:
    - evt.Enriched.IsoCode in ["IL", "US"]
`
	if got != want {
		t.Fatalf("render mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestCsCleanCountries(t *testing.T) {
	t.Run("normalizes case and dedupes", func(t *testing.T) {
		got, err := csCleanCountries([]string{"us", "IL", "il", " US ", ""})
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"US", "IL"}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("got %v want %v", got, want)
		}
	})
	t.Run("rejects non-ISO codes", func(t *testing.T) {
		for _, bad := range []string{"Israel", "I1", "i", "1L", "ILL"} {
			if _, err := csCleanCountries([]string{bad}); err == nil {
				t.Fatalf("expected error for %q", bad)
			}
		}
	})
	t.Run("empty input is valid (feature off)", func(t *testing.T) {
		got, err := csCleanCountries(nil)
		if err != nil || len(got) != 0 {
			t.Fatalf("got %v err %v", got, err)
		}
	})
}

func TestCountryExemptSet_RejectsBadCountry(t *testing.T) {
	params, _ := json.Marshal(map[string]any{"countries": []string{"Israel"}})
	_, err := csCountryExemptSetHandler(context.Background(), params)
	var aerr *agentwire.AgentError
	if e, ok := err.(*agentwire.AgentError); !ok {
		t.Fatalf("expected AgentError, got %T (%v)", err, err)
	} else {
		aerr = e
	}
	if aerr.Code != agentwire.CodeInvalidArgument {
		t.Fatalf("code = %v, want invalid-argument", aerr.Code)
	}
}

// The sync handler validates the whole batch before any cscli call, so
// these run in CI without crowdsec installed.
func TestCountryAllowlistSync_Validation(t *testing.T) {
	cases := []struct {
		name   string
		params map[string]any
	}{
		{"empty adds and removes", map[string]any{"adds": []any{}, "removes": []any{}}},
		{"add with bad cidr", map[string]any{
			"adds": []map[string]string{{"value": "not-a-cidr", "comment": "country:IL"}},
		}},
		{"add with empty value", map[string]any{
			"adds": []map[string]string{{"value": "  ", "comment": "country:IL"}},
		}},
		{"add with short comment", map[string]any{
			"adds": []map[string]string{{"value": "1.2.3.0/24", "comment": "x"}},
		}},
		{"remove with empty value", map[string]any{"removes": []string{" "}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			params, _ := json.Marshal(tc.params)
			_, err := csCountryAllowlistSyncHandler(context.Background(), params)
			aerr, ok := err.(*agentwire.AgentError)
			if !ok {
				t.Fatalf("expected AgentError, got %T (%v)", err, err)
			}
			if aerr.Code != agentwire.CodeInvalidArgument {
				t.Fatalf("code = %v, want invalid-argument (err: %s)", aerr.Code, aerr.Message)
			}
		})
	}
}

func TestCountryAllowlistSync_AcceptsIPAndCIDR(t *testing.T) {
	// Validation must pass plain IPs AND CIDRs. The handler would then call
	// cscli, which CI lacks — so assert only that the error (if any) is NOT
	// an invalid-argument rejection.
	params, _ := json.Marshal(map[string]any{
		"adds": []map[string]string{
			{"value": "1.2.3.4", "comment": "country:IL"},
			{"value": "2001:db8::/32", "comment": "country:IL"},
		},
	})
	_, err := csCountryAllowlistSyncHandler(context.Background(), params)
	if aerr, ok := err.(*agentwire.AgentError); ok && aerr.Code == agentwire.CodeInvalidArgument {
		t.Fatalf("valid IP/CIDR batch rejected: %s", aerr.Message)
	}
}

func TestJabaliManagedAllowlist(t *testing.T) {
	if !jabaliManagedAllowlist("jabali-admin-allowlist") || !jabaliManagedAllowlist("jabali-country-allowlist") {
		t.Fatal("both jabali-managed allowlists must be accepted")
	}
	if jabaliManagedAllowlist("crowdsecurity/whatever") || jabaliManagedAllowlist("") {
		t.Fatal("arbitrary names must be rejected")
	}
}
