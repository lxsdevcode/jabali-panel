package dnsverify

import (
	"context"
	"net"
	"time"
)

// LookupTXTOnServer queries the TXT RRset for name against ONE specific DNS
// server (hostname or IP; port 53 is appended). Used to confirm a freshly
// written ACME challenge is actually being served by the zone's OWN
// authoritative nameservers before telling Let's Encrypt to validate — a
// public-resolver lookup would negative-cache the miss and lie for the
// record's whole SOA-min window, and a blind sleep races the provider's
// propagation (Cloudflare API→edge took >10s on a real zone, JAB-235).
//
// The server hostname itself resolves through the system resolver — it is a
// public name (e.g. chad.ns.cloudflare.com), not a locally-hosted zone, so
// the local-recursor hazard that motivates the rest of this package does not
// apply to it.
func LookupTXTOnServer(ctx context.Context, server, name string) ([]string, error) {
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
			d := &net.Dialer{Timeout: 3 * time.Second}
			return d.DialContext(ctx, "udp", net.JoinHostPort(server, "53"))
		},
	}
	lookupCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return r.LookupTXT(lookupCtx, name)
}
