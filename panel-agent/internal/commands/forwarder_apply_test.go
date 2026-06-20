package commands

import "testing"

func TestBuildExternalSieve(t *testing.T) {
	cases := []struct {
		name      string
		externals []forwarderExternal
		want      string
	}{
		{
			name:      "empty",
			externals: nil,
			want:      "",
		},
		{
			name:      "single keep-copy",
			externals: []forwarderExternal{{Target: "a@example.org", KeepCopy: true}},
			want:      "require [\"copy\"];\nredirect :copy \"a@example.org\";\n",
		},
		{
			name:      "single forward-only (no copy)",
			externals: []forwarderExternal{{Target: "a@example.org", KeepCopy: false}},
			// No "copy" extension required when nothing keeps a copy.
			want: "redirect \"a@example.org\";\n",
		},
		{
			name: "mixed",
			externals: []forwarderExternal{
				{Target: "keep@example.org", KeepCopy: true},
				{Target: "fwd@example.org", KeepCopy: false},
			},
			want: "require [\"copy\"];\nredirect :copy \"keep@example.org\";\nredirect \"fwd@example.org\";\n",
		},
		{
			name: "blank target skipped",
			externals: []forwarderExternal{
				{Target: "", KeepCopy: true},
				{Target: "ok@example.org", KeepCopy: false},
			},
			// hasCopy is true (the blank entry sets it) but the blank
			// redirect line is skipped.
			want: "require [\"copy\"];\nredirect \"ok@example.org\";\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildExternalSieve(tc.externals)
			if got != tc.want {
				t.Errorf("buildExternalSieve mismatch:\n want: %q\n got:  %q", tc.want, got)
			}
		})
	}
}
