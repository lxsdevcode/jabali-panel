package main

import "testing"

// primaryDomainFromManifest picks the is_primary domain, falls back to the
// first, and is empty-safe (JAB-87).
func TestPrimaryDomainFromManifest(t *testing.T) {
	str := func(s string) *string { return &s }

	cases := []struct {
		name string
		in   *string
		want string
	}{
		{"nil", nil, ""},
		{"empty", str(""), ""},
		{"bad json", str("{not json"), ""},
		{"no domains", str(`{"domains":[]}`), ""},
		{"picks is_primary", str(`{"domains":[{"name":"addon.example.com","is_primary":false},{"name":"example.com","is_primary":true}]}`), "example.com"},
		{"falls back to first", str(`{"domains":[{"name":"first.example.com","is_primary":false},{"name":"second.example.com","is_primary":false}]}`), "first.example.com"},
		{"primary with empty name skipped -> first", str(`{"domains":[{"name":"real.example.com","is_primary":false},{"name":"","is_primary":true}]}`), "real.example.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := primaryDomainFromManifest(tc.in); got != tc.want {
				t.Errorf("primaryDomainFromManifest = %q, want %q", got, tc.want)
			}
		})
	}
}
