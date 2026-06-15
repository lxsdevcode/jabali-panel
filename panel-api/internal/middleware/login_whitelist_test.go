package middleware

import "testing"

func TestWhitelistableIP(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{"203.0.113.7", true},          // public v4
		{"2606:4700:4700::1111", true}, // public v6
		{"127.0.0.1", false},           // loopback
		{"::1", false},                 // loopback v6
		{"10.0.0.5", false},            // private
		{"192.168.1.10", false},        // private
		{"172.16.4.2", false},          // private
		{"169.254.1.1", false},         // link-local
		{"0.0.0.0", false},             // unspecified
		{"", false},                    // empty
		{"not-an-ip", false},           // garbage
	}
	for _, c := range cases {
		if got := whitelistableIP(c.ip); got != c.want {
			t.Errorf("whitelistableIP(%q) = %v, want %v", c.ip, got, c.want)
		}
	}
}
