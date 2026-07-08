package models

import "testing"

// JAB-77: read:mail must be a recognised scope, and the read:* wildcard must
// still cover it (so a read-all token works for the mail endpoints too).
func TestReadMailScope(t *testing.T) {
	if !IsAllowedAutomationScope("read:mail") {
		t.Error("read:mail should be an allowed automation scope")
	}
	exact := AutomationScopes{"read:mail"}
	if !exact.Has("read:mail") {
		t.Error("read:mail token should grant read:mail")
	}
	if exact.Has("read:users") {
		t.Error("read:mail must not grant read:users")
	}
	wildcard := AutomationScopes{"read:*"}
	if !wildcard.Has("read:mail") {
		t.Error("read:* should cover read:mail")
	}
}
