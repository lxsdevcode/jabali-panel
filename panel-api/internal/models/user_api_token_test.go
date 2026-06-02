package models

import (
	"testing"
	"time"
)

func TestUserAPIToken_IsActive(t *testing.T) {
	now := time.Now().UTC()
	cases := []struct {
		name string
		tok  *UserAPIToken
		want bool
	}{
		{"nil", nil, false},
		{"no expiry, not revoked", &UserAPIToken{}, true},
		{"revoked", &UserAPIToken{RevokedAt: ptrTime(now.Add(-time.Hour))}, false},
		{"expired", &UserAPIToken{ExpiresAt: ptrTime(now.Add(-time.Second))}, false},
		{"future expiry", &UserAPIToken{ExpiresAt: ptrTime(now.Add(time.Hour))}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.tok.IsActive(now); got != tc.want {
				t.Errorf("IsActive = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestUserAPIScopes_HasNilGrantsEverything(t *testing.T) {
	var s UserAPIScopes
	if !s.Has("dns:write") {
		t.Error("nil scopes should grant everything")
	}
}

func TestUserAPIScopes_HasExact(t *testing.T) {
	s := UserAPIScopes{"dns:read", "mailbox:write"}
	if !s.Has("dns:read") {
		t.Error("exact match should pass")
	}
	if s.Has("dns:write") {
		t.Error("non-listed scope should fail")
	}
}

func TestUserAPIScopes_HasWildcard(t *testing.T) {
	s := UserAPIScopes{"dns:*"}
	if !s.Has("dns:read") {
		t.Error("dns:* should match dns:read")
	}
	if !s.Has("dns:write") {
		t.Error("dns:* should match dns:write")
	}
	if s.Has("mailbox:read") {
		t.Error("dns:* should not match mailbox:read")
	}
}

func ptrTime(t time.Time) *time.Time { return &t }
