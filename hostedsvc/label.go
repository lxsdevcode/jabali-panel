// Package hostedsvc implements the jabalihosted.com free-hostname service
// (JAB-213): PowerDNS-backed IP-derived hostnames + ACME DNS-01 broker for
// Jabali Panel installations. Design + invariants:
// plans/jab213-free-hostname-service.md.
package hostedsvc

import (
	"fmt"
	"net"
	"regexp"
	"strings"
)

// BaseDomain is the PSL-submitted base under which every label lives.
const BaseDomain = "jabalihosted.com"

// LabelFromIP derives the hostname label from the OBSERVED public source
// address of the registering box — cPanel-cprapid style: 192.0.2.7 →
// "192-0-2-7". Labels are never client-chosen (the completed TCP handshake is
// the proof of IP control), and private/special ranges are refused outright:
// an A record into RFC1918 space is a DNS-rebinding lure, not a hostname.
func LabelFromIP(ip net.IP) (string, error) {
	v4 := ip.To4()
	if v4 == nil {
		return "", fmt.Errorf("only IPv4 sources are supported in v1 (got %s)", ip)
	}
	if !v4.IsGlobalUnicast() || v4.IsPrivate() || v4.IsLoopback() || v4.IsLinkLocalUnicast() {
		return "", fmt.Errorf("refusing non-public source address %s", v4)
	}
	return strings.ReplaceAll(v4.String(), ".", "-"), nil
}

// CollisionLabel returns the nth fallback label for a base label when several
// boxes share one public IP (NAT): "1-2-3-4" → "1-2-3-4-b", "1-2-3-4-c", …
// n starts at 1 for the first fallback.
func CollisionLabel(base string, n int) string {
	return fmt.Sprintf("%s-%c", base, 'a'+n)
}

// labelRe is the full shape of every label this service ever creates:
// a dash-encoded IPv4 with an optional single-letter collision suffix.
// Defense-in-depth for anything that round-trips a label through storage.
var labelRe = regexp.MustCompile(`^\d{1,3}-\d{1,3}-\d{1,3}-\d{1,3}(-[a-z])?$`)

// ValidLabel reports whether s is a label this service could have issued.
func ValidLabel(s string) bool {
	if len(s) > 63 || !labelRe.MatchString(s) {
		return false
	}
	// Every octet must round-trip as a real IPv4 octet.
	parts := strings.Split(s, "-")
	for i := 0; i < 4; i++ {
		var o int
		if _, err := fmt.Sscanf(parts[i], "%d", &o); err != nil || o > 255 {
			return false
		}
	}
	return true
}

// FQDN returns the fully-qualified hostname for a label.
func FQDN(label string) string { return label + "." + BaseDomain }
