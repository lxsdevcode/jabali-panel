package main

import (
	"strings"
	"testing"
)

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

func TestRemoveUsernsRemap(t *testing.T) {
	in := []byte(`{"log-driver":"journald","userns-remap":"default"}`)
	out, removed, err := removeUsernsRemap(in)
	if err != nil || !removed {
		t.Fatalf("expected removed=true err=nil, got removed=%v err=%v", removed, err)
	}
	if string(out) == "" || strings.Contains(string(out), "userns-remap") {
		t.Errorf("userns-remap not stripped: %s", out)
	}
	if !strings.Contains(string(out), "log-driver") {
		t.Errorf("other keys lost: %s", out)
	}
	// no-op when absent
	if _, removed2, _ := removeUsernsRemap([]byte(`{"log-driver":"journald"}`)); removed2 {
		t.Error("removed=true on a doc without userns-remap")
	}
}
