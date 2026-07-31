package commands

import "testing"

// A newline in a panel-supplied egress comment would inject arbitrary lines
// into /etc/nftables.d/jabali-per-user-egress.nft, which `nft -f` loads as
// root — or make the file unparseable, which fails the whole per-user egress
// policy open. panel-api rejects these at its boundary; this asserts the agent
// does not rely on that (JAB-188).
func TestSanitiseEgressComment(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, in, want string }{
		{"plain", "office VPN", "office VPN"},
		{"empty", "", ""},
		{"quote becomes apostrophe", `say "hi"`, "say 'hi'"},
		{"newline injection neutralised", "ok\n    ip daddr 0.0.0.0/0 accept", "ok     ip daddr 0.0.0.0/0 accept"},
		{"carriage return", "a\rb", "a b"},
		{"NUL and control bytes", "a\x00b\x07c", "a b c"},
		{"backslash escape", `a\nb`, "a/nb"},
		{"surrounding space trimmed", "  padded  ", "padded"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sanitiseEgressComment(tc.in); got != tc.want {
				t.Errorf("sanitiseEgressComment(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSanitiseEgressComment_NeverEmitsNewlineOrQuote(t *testing.T) {
	t.Parallel()
	got := sanitiseEgressComment("x\n\"y\"\r\nz")
	for _, r := range got {
		if r == '\n' || r == '\r' || r == '"' {
			t.Fatalf("sanitised comment still contains %q: %q", r, got)
		}
	}
}

func TestSanitiseEgressComment_CapsLength(t *testing.T) {
	t.Parallel()
	long := make([]byte, 500)
	for i := range long {
		long[i] = 'a'
	}
	if got := sanitiseEgressComment(string(long)); len(got) != 200 {
		t.Errorf("length = %d, want 200", len(got))
	}
}
