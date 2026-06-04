// Package dnsverify provides authoritative-resolution helpers that
// bypass /etc/hosts AND the host's local recursor. We need this in two
// places that have the same hazard: the panel-cert routability gate
// (would the LE validator see what the panel sees?) and the per-tenant
// SAN filter (would the LE validator resolve `mail.<tenant>` if we
// asked it to?).
//
// On a typical jabali host /etc/resolv.conf points at the local
// pdns-recursor which has pdns-auth as its forward path. The auth
// server is authoritative for every domain hosted on this server,
// including tenant zones — so a local lookup of `mail.<tenant>`
// succeeds even when the tenant delegates DNS to a foreign registrar
// and that registrar has no `mail` record. Let's Encrypt queries the
// real authoritative nameservers, gets NXDOMAIN, and the challenge
// fails with acme:error:dns. The fix is to ask a public resolver
// directly so we see the world's view.
//
// Resolver list: Cloudflare, Quad9, Google. Each tried over UDP first
// then TCP so carriers that block outbound UDP/53 (some LXC providers
// do) still succeed.
package dnsverify

import (
	"context"
	"net"
	"time"
)

// externalResolvers is the ordered list of public resolvers the
// shadow-guard retries against. Cloudflare, Quad9, Google.
var externalResolvers = []string{"1.1.1.1:53", "9.9.9.9:53", "8.8.8.8:53"}

// LookupHostExternal walks externalResolvers (UDP then TCP per host)
// and returns the first non-empty LookupHost result. Returns nil if
// every attempt fails. Each attempt has a tight per-resolver timeout
// so a single unreachable resolver can't stall a reconciler tick.
func LookupHostExternal(ctx context.Context, host string) []string {
	for _, addr := range externalResolvers {
		for _, proto := range []string{"udp", "tcp"} {
			r := newDirectResolver(addr, proto)
			lookupCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			out, err := r.LookupHost(lookupCtx, host)
			cancel()
			if err == nil && len(out) > 0 {
				return out
			}
		}
	}
	return nil
}

// newDirectResolver returns a pure-Go resolver that bypasses
// /etc/hosts by dialing the given DNS server:port over the given
// transport ("udp" or "tcp") directly.
func newDirectResolver(serverAddr, proto string) *net.Resolver {
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
			d := &net.Dialer{Timeout: 3 * time.Second}
			return d.DialContext(ctx, proto, serverAddr)
		},
	}
}
