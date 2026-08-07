package hostedsvc

import (
	"net"
	"testing"
)

func TestLabelFromIP(t *testing.T) {
	tests := []struct {
		ip      string
		want    string
		wantErr bool
	}{
		{"192.0.2.7", "192-0-2-7", false},
		{"182.54.236.100", "182-54-236-100", false},
		{"192.168.100.165", "", true}, // RFC1918 — rebinding lure, refused
		{"10.0.3.14", "", true},
		{"127.0.0.1", "", true},
		{"169.254.1.1", "", true},
		{"2001:db8::1", "", true}, // v6 not in v1
	}
	for _, tc := range tests {
		got, err := LabelFromIP(net.ParseIP(tc.ip))
		if tc.wantErr != (err != nil) {
			t.Errorf("%s: err = %v, wantErr %v", tc.ip, err, tc.wantErr)
			continue
		}
		if got != tc.want {
			t.Errorf("%s: label = %q, want %q", tc.ip, got, tc.want)
		}
	}
}

func TestCollisionLabel(t *testing.T) {
	if got := CollisionLabel("1-2-3-4", 1); got != "1-2-3-4-b" {
		t.Errorf("first fallback = %q", got)
	}
	if got := CollisionLabel("1-2-3-4", 2); got != "1-2-3-4-c" {
		t.Errorf("second fallback = %q", got)
	}
}

func TestValidLabel(t *testing.T) {
	for _, ok := range []string{"192-0-2-7", "1-2-3-4-b", "255-255-255-255"} {
		if !ValidLabel(ok) {
			t.Errorf("%q should be valid", ok)
		}
	}
	for _, bad := range []string{"", "www", "192-0-2", "999-0-2-7", "192-0-2-7-bb", "192-0-2-7.evil", "a-b-c-d"} {
		if ValidLabel(bad) {
			t.Errorf("%q should be invalid", bad)
		}
	}
}
