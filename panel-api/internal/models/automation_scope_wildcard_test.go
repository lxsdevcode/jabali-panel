package models

import (
	"reflect"
	"testing"
)

// JAB-84: read:* and individual read scopes are mutually exclusive.
func TestReadWildcardConflict(t *testing.T) {
	if !HasReadWildcardConflict([]string{"read:*", "read:domains"}) {
		t.Error("read:* + read:domains should conflict")
	}
	if HasReadWildcardConflict([]string{"read:*"}) {
		t.Error("read:* alone is fine")
	}
	if HasReadWildcardConflict([]string{"read:domains", "read:users"}) {
		t.Error("explicit-only is fine")
	}
	// normalize drops children under the wildcard
	got := NormalizeScopes([]string{"read:*", "read:domains", "read:users"})
	if !reflect.DeepEqual(got, []string{"read:*"}) {
		t.Errorf("normalize = %v, want [read:*]", got)
	}
	// no wildcard → unchanged
	in := []string{"read:domains", "read:status"}
	if got := NormalizeScopes(in); !reflect.DeepEqual(got, in) {
		t.Errorf("normalize explicit changed it: %v", got)
	}
}
