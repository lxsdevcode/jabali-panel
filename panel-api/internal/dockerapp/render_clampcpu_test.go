package dockerapp

import "testing"

func TestClampCPULimit(t *testing.T) {
	cases := []struct {
		name     string
		cpu      string
		hostCPUs int
		want     string
	}{
		{"over budget on 1-cpu host clamps", "2.0", 1, "1.00"}, // GH#178
		{"over budget on 2-cpu host clamps", "4", 2, "2.00"},
		{"in range passes through", "1.0", 4, "1.0"},
		{"equal to host passes through", "2.0", 2, "2.0"},
		{"empty passes through", "", 1, ""},
		{"unparseable passes through", "all", 1, "all"},
		{"zero host cpus is a no-op", "2.0", 0, "2.0"},
		{"fractional in range passes through", "0.50", 1, "0.50"},
		{"fractional over budget clamps", "1.50", 1, "1.00"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := clampCPULimit(c.cpu, c.hostCPUs); got != c.want {
				t.Errorf("clampCPULimit(%q, %d) = %q, want %q", c.cpu, c.hostCPUs, got, c.want)
			}
		})
	}
}
