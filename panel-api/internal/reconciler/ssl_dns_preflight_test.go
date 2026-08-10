package reconciler

// GH #896 follow-up. A freshly created (sub)domain routinely has no public
// A/AAAA record yet, and firing certbot at it guarantees an NXDOMAIN failure
// that surfaces in the panel as a raw internal error — the reporter's
// second complaint on retest. The pre-flight parks the cert with a calm
// "waiting for DNS" reason instead of attempting ACME.
//
// Three properties matter, and each has a failure mode this file pins:
//
//  1. No records (definitive) → NO ssl.issue call, calm reason, and the
//     retry count is NOT consumed — no LE attempt happened, so a slow DNS
//     setup must not burn through acmeMaxRetries and strand on "failed".
//  2. Resolves → ACME proceeds exactly as before.
//  3. Inconclusive (all external resolvers unreachable) → ACME proceeds.
//     Parking on an inconclusive answer would mean an outbound-:53 outage
//     on OUR side silently stops all issuance forever.

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestSSLPreflight_NoDNS_ParksWithoutBurningRetries(t *testing.T) {
	r, ag, sc, dom := sslWebrootFixture(t, nil)
	sc.byDomain[dom.ID].RetryCount = 3
	r.dnsPreflight = func(context.Context, string) ([]string, bool) {
		return nil, true // definitive: public DNS has no record
	}

	r.reconcileSSLForDomain(context.Background(), dom)

	if agentCalled(ag, "ssl.issue") {
		t.Fatal("ssl.issue was attempted for a name with no public DNS record — guaranteed NXDOMAIN, scary error, wasted LE quota")
	}
	cert := sc.byDomain[dom.ID]
	if cert.RetryCount != 3 {
		t.Errorf("retry count %d, want 3 (unchanged) — a DNS wait is not an ACME attempt and must not consume the cap", cert.RetryCount)
	}
	if cert.LastError == nil || !strings.Contains(*cert.LastError, "waiting for DNS") {
		t.Errorf("last error should be the calm waiting-for-DNS reason, got %v", cert.LastError)
	}
	if cert.NextRetryAt == nil || time.Until(*cert.NextRetryAt) > dnsWaitRetryDelay+time.Minute {
		t.Errorf("next retry should be ~%s out, got %v", dnsWaitRetryDelay, cert.NextRetryAt)
	}
}

func TestSSLPreflight_Resolves_AttemptsACME(t *testing.T) {
	r, ag, _, dom := sslWebrootFixture(t, nil)
	r.dnsPreflight = func(context.Context, string) ([]string, bool) {
		return []string{"192.0.2.1"}, true
	}

	r.reconcileSSLForDomain(context.Background(), dom)

	if !agentCalled(ag, "ssl.issue") {
		t.Fatal("a resolving domain must proceed to the ACME attempt")
	}
}

func TestSSLPreflight_InconclusiveLookup_AttemptsACME(t *testing.T) {
	r, ag, _, dom := sslWebrootFixture(t, nil)
	r.dnsPreflight = func(context.Context, string) ([]string, bool) {
		return nil, false // every external resolver unreachable
	}

	r.reconcileSSLForDomain(context.Background(), dom)

	if !agentCalled(ag, "ssl.issue") {
		t.Fatal("an INCONCLUSIVE lookup must fall through to ACME — parking on it turns a resolver outage on our side into a permanent issuance freeze")
	}
}
