package models

import (
	"reflect"
	"testing"
)

// JAB-140: read:* must NEVER satisfy a write scope, and write:* must never
// satisfy a delete scope — the whole security model rests on this.
func TestScopes_NoCrossFamilyLeak(t *testing.T) {
	cases := []struct {
		granted AutomationScopes
		want    string
		has     bool
	}{
		{AutomationScopes{"read:*"}, "read:domains", true},
		{AutomationScopes{"read:*"}, "write:services", false}, // no read->write leak
		{AutomationScopes{"read:*"}, "delete:users", false},
		{AutomationScopes{"write:*"}, "write:backups", true},
		{AutomationScopes{"write:*"}, "delete:users", false}, // no write->delete leak
		{AutomationScopes{"write:*"}, "read:users", false},   // no write->read leak
		{AutomationScopes{"write:services"}, "write:services", true},
		{AutomationScopes{"write:services"}, "write:users", false},
		{AutomationScopes{"delete:*"}, "delete:domains", true},
		{AutomationScopes{"delete:*"}, "write:domains", false},
	}
	for _, c := range cases {
		if got := c.granted.Has(c.want); got != c.has {
			t.Errorf("%v.Has(%q) = %v, want %v", c.granted, c.want, got, c.has)
		}
	}
}

func TestAllowedScopes_IncludeWriteAndDelete(t *testing.T) {
	for _, s := range []string{"write:*", "write:services", "write:users", "write:domains", "write:cache", "write:backups", "delete:*", "delete:users", "delete:domains"} {
		if !IsAllowedAutomationScope(s) {
			t.Errorf("scope %q should be allowed", s)
		}
	}
	if IsAllowedAutomationScope("write:bogus") {
		t.Error("write:bogus must not be allowed")
	}
}

func TestWildcardConflict_PerFamily(t *testing.T) {
	if !HasWildcardConflict([]string{"write:*", "write:users"}) {
		t.Error("write:* + write:users should conflict")
	}
	if !HasWildcardConflict([]string{"read:*", "read:domains"}) {
		t.Error("read:* + read:domains should conflict (JAB-84)")
	}
	// Cross-family is NOT a conflict: read:* + write:users is a valid combo.
	if HasWildcardConflict([]string{"read:*", "write:users"}) {
		t.Error("read:* + write:users must NOT conflict")
	}
	if HasWildcardConflict([]string{"write:services", "write:users"}) {
		t.Error("two explicit write scopes must NOT conflict")
	}
}

func TestNormalizeScopes_PerFamily(t *testing.T) {
	// write:* absorbs explicit write scopes but leaves read:domains + delete:* alone.
	got := NormalizeScopes([]string{"write:*", "write:users", "write:cache", "read:domains", "delete:*"})
	want := []string{"write:*", "read:domains", "delete:*"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("normalize = %v, want %v", got, want)
	}
	// No wildcard → unchanged.
	in := []string{"write:services", "read:users"}
	if got := NormalizeScopes(in); !reflect.DeepEqual(got, in) {
		t.Errorf("normalize without wildcard changed input: %v", got)
	}
}
