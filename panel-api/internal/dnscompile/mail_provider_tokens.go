package dnscompile

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// M365 tenant: a label or <label>.onmicrosoft.com. We normalise to the
	// full onmicrosoft.com form. The label is the only operator-supplied
	// part that flows into a CNAME content, so it is strictly bounded.
	m365LabelRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
)

// NormaliseM365Onmicrosoft validates + canonicalises the M365 tenant token.
// Accepts "contoso" or "contoso.onmicrosoft.com"; returns
// "contoso.onmicrosoft.com". Empty in -> empty out (no DKIM CNAMEs emitted).
func NormaliseM365Onmicrosoft(in string) (string, error) {
	in = strings.TrimSpace(strings.ToLower(in))
	if in == "" {
		return "", nil
	}
	label := strings.TrimSuffix(in, ".onmicrosoft.com")
	if !m365LabelRe.MatchString(label) {
		return "", fmt.Errorf("invalid Microsoft 365 tenant %q (expected a label like 'contoso' or 'contoso.onmicrosoft.com')", in)
	}
	return label + ".onmicrosoft.com", nil
}

// ValidateGoogleDKIM checks the operator-pasted Google Workspace DKIM TXT
// value is safe to embed in a DNS TXT record: printable, no quote/newline/
// control bytes (which would break the zone), bounded length.
func ValidateGoogleDKIM(in string) (string, error) {
	in = strings.TrimSpace(in)
	if in == "" {
		return "", nil
	}
	if len(in) > 2048 {
		return "", fmt.Errorf("Google DKIM value too long (%d > 2048)", len(in))
	}
	for _, r := range in {
		if r == '"' || r == '\n' || r == '\r' || r < 0x20 {
			return "", fmt.Errorf("Google DKIM value contains an illegal character")
		}
	}
	return in, nil
}
