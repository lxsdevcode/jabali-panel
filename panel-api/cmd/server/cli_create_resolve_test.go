package main

import "testing"

func TestResolveCreateIdentity(t *testing.T) {
	sp := func(s string) *string { return &s }
	eq := func(a, b *string) bool {
		if a == nil || b == nil {
			return a == b
		}
		return *a == *b
	}
	cases := []struct {
		name, username, email string
		admin                 bool
		host                  string
		wantUser              *string
		wantEmail             string
		wantErr               bool
	}{
		{"username only synthesizes email", "alice", "", false, "panel.example.com", sp("alice"), "alice@panel.example.com", false},
		{"email only derives username", "", "bob@x.com", false, "panel.example.com", sp("bob"), "bob@x.com", false},
		{"username + email both kept", "carol", "carol@x.com", false, "panel.example.com", sp("carol"), "carol@x.com", false},
		{"neither is an error", "", "", false, "panel.example.com", nil, "", true},
		{"invalid username is an error", "Bad User", "", false, "h", nil, "", true},
		{"admin may stay username-less", "", "admin@x.com", true, "h", nil, "admin@x.com", false},
		{"empty host falls back to localhost", "dan", "", false, "", sp("dan"), "dan@localhost.localdomain", false},
	}
	for _, c := range cases {
		u, e, err := resolveCreateIdentity(c.username, c.email, c.admin, c.host)
		if (err != nil) != c.wantErr {
			t.Errorf("%s: err=%v wantErr=%v", c.name, err, c.wantErr)
			continue
		}
		if c.wantErr {
			continue
		}
		if !eq(u, c.wantUser) || e != c.wantEmail {
			t.Errorf("%s: got (%v,%q) want (%v,%q)", c.name, u, e, c.wantUser, c.wantEmail)
		}
	}
}
