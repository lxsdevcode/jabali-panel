package api

import "testing"

func TestClampDockerMemory(t *testing.T) {
	cases := []struct {
		in   string
		cap  uint32
		want string
	}{
		{"512m", 0, "512m"},         // cap 0 = unlimited
		{"512m", 1024, "512m"},      // under cap
		{"2g", 1024, "1024m"},       // over cap -> capped
		{"1024m", 512, "512m"},      // over cap
		{"256m", 512, "256m"},       // under
		{"garbage", 512, "garbage"}, // unparseable -> unchanged
		{"", 512, ""},               // empty -> unchanged
	}
	for _, c := range cases {
		if got := clampDockerMemory(c.in, c.cap); got != c.want {
			t.Errorf("clampDockerMemory(%q,%d)=%q want %q", c.in, c.cap, got, c.want)
		}
	}
}

func TestClampDockerCPU(t *testing.T) {
	cases := []struct {
		in   string
		cap  uint32
		want string
	}{
		{"0.5", 0, "0.5"},   // unlimited
		{"0.5", 100, "0.5"}, // under 1 core
		{"2", 100, "1"},     // over 1 core -> 1
		{"2", 50, "0.5"},    // over 0.5 core
		{"x", 50, "x"},      // unparseable
	}
	for _, c := range cases {
		if got := clampDockerCPU(c.in, c.cap); got != c.want {
			t.Errorf("clampDockerCPU(%q,%d)=%q want %q", c.in, c.cap, got, c.want)
		}
	}
}

func TestClampPIDs(t *testing.T) {
	cases := []struct {
		in   int
		cap  uint32
		want int
	}{
		{100, 0, 100},   // unlimited
		{100, 200, 100}, // under
		{300, 200, 200}, // over
		{0, 200, 200},   // unlimited request forced to cap
		{-1, 200, 200},
	}
	for _, c := range cases {
		if got := clampPIDs(c.in, c.cap); got != c.want {
			t.Errorf("clampPIDs(%d,%d)=%d want %d", c.in, c.cap, got, c.want)
		}
	}
}
