package models

import "testing"

func TestPoolSlug(t *testing.T) {
	cases := []struct {
		user, ver string
		isDefault bool
		wantSlug  string
		wantSock  string
	}{
		{"alice", "8.4", true, "alice", "/run/php/jabali-alice/fpm.sock"},
		{"alice", "8.2", false, "alice-php8.2", "/run/php/jabali-alice-php8.2/fpm.sock"},
		{"bob_1", "7.4", false, "bob_1-php7.4", "/run/php/jabali-bob_1-php7.4/fpm.sock"},
	}
	for _, c := range cases {
		if got := PoolSlug(c.user, c.ver, c.isDefault); got != c.wantSlug {
			t.Errorf("PoolSlug(%q,%q,%v) = %q, want %q", c.user, c.ver, c.isDefault, got, c.wantSlug)
		}
		if got := PoolSocketPath(c.user, c.ver, c.isDefault); got != c.wantSock {
			t.Errorf("PoolSocketPath(%q,%q,%v) = %q, want %q", c.user, c.ver, c.isDefault, got, c.wantSock)
		}
	}
}
