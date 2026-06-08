package commands

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"git.linux-hosting.co.il/shukivaknin/jabali2/agentwire"
)

func TestMailSANHostnames_Order(t *testing.T) {
	got := mailSANHostnames("Example.COM")
	want := []string{"mail.example.com", "autoconfig.example.com", "autodiscover.example.com", "mta-sts.example.com"}
	assert.Equal(t, want, got, "order must be stable + lowercased")
}

func TestSSLMailIssue_RejectsEmptyDomain(t *testing.T) {
	raw, _ := json.Marshal(sslMailIssueParams{Domain: "  "})
	_, err := sslMailIssueHandler(context.Background(), raw)
	require.Error(t, err)
	var aerr *agentwire.AgentError
	require.ErrorAs(t, err, &aerr)
	assert.Equal(t, agentwire.CodeInvalidArgument, aerr.Code)
}

func TestSSLMailIssue_RejectsInvalidDomain(t *testing.T) {
	raw, _ := json.Marshal(sslMailIssueParams{Domain: "../etc/passwd"})
	_, err := sslMailIssueHandler(context.Background(), raw)
	require.Error(t, err)
	var aerr *agentwire.AgentError
	require.ErrorAs(t, err, &aerr)
	assert.Equal(t, agentwire.CodeInvalidArgument, aerr.Code)
}

func TestSSLMailIssue_RequiresPublicIPWhenNotSkipped(t *testing.T) {
	raw, _ := json.Marshal(sslMailIssueParams{Domain: "example.com"})
	_, err := sslMailIssueHandler(context.Background(), raw)
	require.Error(t, err)
	var aerr *agentwire.AgentError
	require.ErrorAs(t, err, &aerr)
	assert.Equal(t, agentwire.CodeInvalidArgument, aerr.Code)
}

func TestSSLMailIssue_SkipDNS_StillRequiresEmail(t *testing.T) {
	raw, _ := json.Marshal(sslMailIssueParams{
		Domain:  "example.com",
		SkipDNS: true,
	})
	_, err := sslMailIssueHandler(context.Background(), raw)
	require.Error(t, err)
	var aerr *agentwire.AgentError
	require.ErrorAs(t, err, &aerr)
	assert.Equal(t, agentwire.CodeInvalidArgument, aerr.Code)
}

func TestScanMailSANDNS_AllNonResolvableNoneMatch(t *testing.T) {
	// A bunch of guaranteed-NX hostnames. Even if a flaky network
	// returns some glue address, none of these can ever equal the
	// canary publicIP below, so no row may report Matches=true.
	sans := []string{
		"mail.this-domain-cannot-exist-jabali-test.invalid",
		"autoconfig.this-domain-cannot-exist-jabali-test.invalid",
		"autodiscover.this-domain-cannot-exist-jabali-test.invalid",
		"mta-sts.this-domain-cannot-exist-jabali-test.invalid",
	}
	dns := scanMailSANDNS(context.Background(), sans, "203.0.113.42", nil)
	require.Len(t, dns, len(sans))
	for _, h := range sans {
		assert.False(t, dns[h].Matches, "%s must not match the canary IP", h)
	}
}

func TestSelectEffectiveSANs_MailOnlyWhenAuxPointElsewhere(t *testing.T) {
	// GH #132 shape: mail.<d> points at us, the three autoconfig
	// helpers point at a separate webmail host. The cert must drop
	// the non-matching aux names instead of bundling them.
	all := mailSANHostnames("example.com")
	dns := map[string]dnsScan{
		all[0]: {Hostname: all[0], Matches: true},
		all[1]: {Hostname: all[1], Matches: false},
		all[2]: {Hostname: all[2], Matches: false},
		all[3]: {Hostname: all[3], Matches: false},
	}
	got := selectEffectiveSANs(all, dns)
	assert.Equal(t, []string{"mail.example.com"}, got)
}

func TestSelectEffectiveSANs_AllMatchKeepsEverything(t *testing.T) {
	all := mailSANHostnames("example.com")
	dns := map[string]dnsScan{}
	for _, h := range all {
		dns[h] = dnsScan{Hostname: h, Matches: true}
	}
	got := selectEffectiveSANs(all, dns)
	assert.Equal(t, all, got, "all-resolving SANs must all be requested")
}

func TestSelectEffectiveSANs_PartialMatchKeepsOrder(t *testing.T) {
	all := mailSANHostnames("example.com")
	dns := map[string]dnsScan{
		all[0]: {Hostname: all[0], Matches: true},
		all[1]: {Hostname: all[1], Matches: false},
		all[2]: {Hostname: all[2], Matches: true},
		all[3]: {Hostname: all[3], Matches: false},
	}
	got := selectEffectiveSANs(all, dns)
	assert.Equal(t, []string{"mail.example.com", "autodiscover.example.com"}, got)
}

// TestScanMailSANDNS_PublicVsLocal asserts the scan reflects PUBLIC DNS:
// a SAN that resolves to publicIP only via a stub "public" answer is kept;
// one that publicly NXDOMAINs (queried, empty) is dropped even though the
// box would have resolved it locally (GH #132); and when public resolvers
// are unreachable (queried=false) it falls back to the box resolver.
func TestScanMailSANDNS_PublicVsLocal(t *testing.T) {
	const ip = "203.0.113.42"
	sans := mailSANHostnames("example.com") // mail., autoconfig., autodiscover., mta-sts.
	stub := func(_ context.Context, host string) ([]string, bool) {
		switch host {
		case "mail.example.com":
			return []string{ip}, true // public points at us -> keep
		case "autoconfig.example.com":
			return []string{ip}, true // delegated tenant: public also at us -> keep
		case "autodiscover.example.com":
			return nil, true // public NXDOMAIN -> drop (the GH#132 case)
		default: // mta-sts.example.com
			return []string{"198.51.100.9"}, true // elsewhere -> drop
		}
	}
	dns := scanMailSANDNS(context.Background(), sans, ip, stub)
	if !dns["mail.example.com"].Matches {
		t.Errorf("mail.<d> must match (public points at us)")
	}
	if !dns["autoconfig.example.com"].Matches {
		t.Errorf("autoconfig must match when public resolves to us (don't over-drop delegated tenants)")
	}
	if dns["autodiscover.example.com"].Matches {
		t.Errorf("autodiscover must NOT match on public NXDOMAIN (GH#132)")
	}
	if dns["mta-sts.example.com"].Matches {
		t.Errorf("mta-sts pointing elsewhere must NOT match")
	}

	// selectEffectiveSANs then keeps mail + autoconfig, drops the rest.
	eff := selectEffectiveSANs(sans, dns)
	want := map[string]bool{"mail.example.com": true, "autoconfig.example.com": true}
	if len(eff) != 2 {
		t.Fatalf("effective SANs: got %v, want exactly mail+autoconfig", eff)
	}
	for _, h := range eff {
		if !want[h] {
			t.Errorf("unexpected SAN kept: %s", h)
		}
	}
}
