package main

import "testing"

func TestUIDMapIsUnprivileged(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"identity host/VM/priv-lxc", "         0          0 4294967295\n", false},
		{"identity no padding", "0 0 4294967295", false},
		{"unprivileged lxc remap", "         0     100000      65536\n", true},
		{"unprivileged single id", "0 1000 1", true},
		{"empty", "", false},
		{"malformed", "garbage line", false},
	}
	for _, c := range cases {
		if got := uidMapIsUnprivileged(c.in); got != c.want {
			t.Errorf("%s: uidMapIsUnprivileged(%q)=%v want %v", c.name, c.in, got, c.want)
		}
	}
}
