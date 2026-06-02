package commands

// dotSAN maps a public-resolver IP (literal, as accepted by netip.ParseAddr)
// to its TLS server name. systemd-resolved consumes this via the IP#SAN
// drop-in syntax (`DNS=1.1.1.1#cloudflare-dns.com`); the SAN tells resolved
// what name to validate the upstream certificate against. Coverage is the
// presets surfaced in the panel UI plus their IPv6 counterparts — every
// entry is a DoT-supporting resolver as of 2026.
//
// Why hardcode: the panel's "DNS Resolvers" UI is a finite, vetted preset
// list (Cloudflare / Google / Quad9 / OpenDNS / AdGuard / etc); adding a
// new preset is a deliberate code change anyway. Custom IPs entered by an
// operator stay un-SAN'd in the drop-in — they still get DoT via
// `DNSOverTLS=opportunistic` (best-effort TLS with plain fallback) so a
// non-DoT upstream still works on UDP-OK hosts.
var dotSAN = map[string]string{
	// Cloudflare 1.1.1.1 (cloudflare-dns.com)
	"1.1.1.1":               "cloudflare-dns.com",
	"1.0.0.1":               "cloudflare-dns.com",
	"2606:4700:4700::1111":  "cloudflare-dns.com",
	"2606:4700:4700::1001":  "cloudflare-dns.com",
	// Cloudflare Family (security) 1.1.1.2 / Family (no adult) 1.1.1.3
	"1.1.1.2":               "security.cloudflare-dns.com",
	"1.0.0.2":               "security.cloudflare-dns.com",
	"1.1.1.3":               "family.cloudflare-dns.com",
	"1.0.0.3":               "family.cloudflare-dns.com",
	// Google Public DNS 8.8.8.8 (dns.google)
	"8.8.8.8":               "dns.google",
	"8.8.4.4":               "dns.google",
	"2001:4860:4860::8888":  "dns.google",
	"2001:4860:4860::8844":  "dns.google",
	// Quad9 (DNSSEC + malware filter)
	"9.9.9.9":               "dns.quad9.net",
	"149.112.112.112":       "dns.quad9.net",
	"2620:fe::fe":           "dns.quad9.net",
	"2620:fe::9":            "dns.quad9.net",
	// OpenDNS
	"208.67.222.222":        "dns.opendns.com",
	"208.67.220.220":        "dns.opendns.com",
	"2620:119:35::35":       "dns.opendns.com",
	"2620:119:53::53":       "dns.opendns.com",
	// AdGuard DNS (default)
	"94.140.14.14":          "dns.adguard-dns.com",
	"94.140.15.15":          "dns.adguard-dns.com",
	"2a10:50c0::ad1:ff":     "dns.adguard-dns.com",
	"2a10:50c0::ad2:ff":     "dns.adguard-dns.com",
	// Mullvad
	"194.242.2.2":           "dns.mullvad.net",
	"194.242.2.3":           "adblock.dns.mullvad.net",
	// Control D (default unfiltered)
	"76.76.2.0":             "freedns.controld.com",
	"76.76.10.0":            "freedns.controld.com",
}

// dotSuffixFor returns the SAN annotation to append to an IP literal in
// the resolved.conf drop-in, or empty if the IP is not in the known-DoT
// map (custom operator-supplied IP, RFC1918 corp resolver, etc.).
func dotSuffixFor(ip string) string {
	return dotSAN[ip]
}
