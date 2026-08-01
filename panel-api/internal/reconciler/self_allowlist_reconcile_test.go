package reconciler

import (
	"reflect"
	"testing"
)

// JAB-195. The set that gets allowlisted decides whether the host's own
// loopback traffic is WAF-scored, so the filtering is worth pinning directly.
func TestSelfPublicIPs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		v4, v6 string
		want   []string
	}{
		{
			name: "the reported production shape",
			v4:   "182.54.236.64",
			want: []string{"182.54.236.64"},
		},
		{
			name: "v4 and v6 both allowlisted",
			v4:   "182.54.236.64",
			v6:   "2a01:4f8:1c1c:1::1",
			want: []string{"182.54.236.64", "2a01:4f8:1c1c:1::1"},
		},
		{
			name: "unset columns produce nothing to do",
			want: []string{},
		},
		// A blank or placeholder public_ipv4 is normal on a freshly installed
		// host. Filtering here keeps the reconciler from calling cscli with
		// garbage every tick and logging a failed add each time.
		{
			name: "malformed value is dropped rather than sent to cscli",
			v4:   "not-an-ip",
			want: []string{},
		},
		{
			name: "whitespace is tolerated",
			v4:   "  182.54.236.64  ",
			want: []string{"182.54.236.64"},
		},
		// Loopback never appears as an AppSec source, so allowlisting it only
		// widens the entry list for no benefit.
		{
			name: "loopback dropped",
			v4:   "127.0.0.1",
			v6:   "::1",
			want: []string{},
		},
		{
			name: "unspecified dropped",
			v4:   "0.0.0.0",
			v6:   "::",
			want: []string{},
		},
		// Same address in both columns must not produce two identical entries.
		{
			name: "duplicates collapse",
			v4:   "182.54.236.64",
			v6:   "182.54.236.64",
			want: []string{"182.54.236.64"},
		},
		// Equivalent v6 spellings normalise to one entry.
		{
			name: "v6 spellings normalise",
			v4:   "2A01:4F8:1C1C:1:0:0:0:1",
			v6:   "2a01:4f8:1c1c:1::1",
			want: []string{"2a01:4f8:1c1c:1::1"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := selfPublicIPs(tc.v4, tc.v6)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("selfPublicIPs(%q, %q) = %v, want %v", tc.v4, tc.v6, got, tc.want)
			}
		})
	}
}

// The reason string lands in `cscli allowlists inspect` output, where it is the
// only clue an operator has about why the entry exists. Pinned so it stays
// self-explanatory and traceable rather than drifting to something terse.
func TestSelfAllowlistReason_IsTraceable(t *testing.T) {
	t.Parallel()
	for _, want := range []string{"jabali", "JAB-195"} {
		if !contains(selfAllowlistReason, want) {
			t.Errorf("allowlist reason %q should mention %q so an operator can trace it",
				selfAllowlistReason, want)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}
