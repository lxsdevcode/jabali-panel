package commands

import (
	"strings"
	"testing"
)

// JAB-29: mail JMAP methods (Mailbox/get, Mailbox/query, Email/*) must advertise
// urn:ietf:params:jmap:mail in the `using` set or Stalwart rejects them with
// unknownMethod, which silently aborted mailbox message import at "resolve
// inbox". Lock the capability into the request builder.
func TestJmapUsingWith_IncludesMailCapability(t *testing.T) {
	const mailCap = "urn:ietf:params:jmap:mail"
	using := jmapUsingWith(mailCap)

	has := func(want string) bool {
		for _, u := range using {
			if u == want {
				return true
			}
		}
		return false
	}

	if !has(mailCap) {
		t.Errorf("using set %v missing %q", using, mailCap)
	}
	if !has("urn:ietf:params:jmap:core") {
		t.Errorf("using set %v dropped the base core capability", using)
	}
	// The extra cap must be appended, not replace the base set.
	if len(using) <= len(jmapUsing) {
		t.Errorf("using set %v did not append to base %v", using, jmapUsing)
	}
	// Sanity: no accidental empty entries.
	for _, u := range using {
		if strings.TrimSpace(u) == "" {
			t.Errorf("using set has an empty capability: %v", using)
		}
	}
}
