package commands

import (
	"reflect"
	"testing"
)

// TestKeywordsToMaildirFlags — export side: JMAP keywords → sorted
// Maildir info-flag letters (D,F,R,S order). Regression for the
// flags-lost defect (\Flagged/\Answered/\Draft were stripped, only
// \Seen survived via cur/ vs new/).
func TestKeywordsToMaildirFlags(t *testing.T) {
	cases := []struct {
		name string
		kw   map[string]bool
		want string
	}{
		{"empty", map[string]bool{}, ""},
		{"nil", nil, ""},
		{"seen only", map[string]bool{"$seen": true}, "S"},
		{"flagged only", map[string]bool{"$flagged": true}, "F"},
		{"answered only", map[string]bool{"$answered": true}, "R"},
		{"draft only", map[string]bool{"$draft": true}, "D"},
		{
			"all four sorted",
			map[string]bool{"$seen": true, "$flagged": true, "$answered": true, "$draft": true},
			"DFRS",
		},
		{
			"flagged+answered, unseen",
			map[string]bool{"$flagged": true, "$answered": true},
			"FR",
		},
		{
			"unknown keyword ignored",
			map[string]bool{"$seen": true, "$forwarded": true, "$junk": true},
			"S",
		},
	}
	for _, c := range cases {
		if got := keywordsToMaildirFlags(c.kw); got != c.want {
			t.Errorf("%s: keywordsToMaildirFlags = %q, want %q", c.name, got, c.want)
		}
	}
}

// TestMaildirInfoFlags — import side: pull the ":2,<flags>" suffix off a
// filename. new/ files (no suffix) must report hasInfo=false so the
// caller falls back to the slot.
func TestMaildirInfoFlags(t *testing.T) {
	cases := []struct {
		name      string
		filename  string
		wantFlags string
		wantHas   bool
	}{
		{"new file, bare", "169.jabali,4096", "", false},
		{"cur seen", "169.jabali,4096:2,S", "S", true},
		{"cur all flags", "169.jabali,4096:2,DFRS", "DFRS", true},
		{"cur flagged unseen", "169.jabali,4096:2,F", "F", true},
		{"info marker, empty flags", "169.jabali,4096:2,", "", true},
		// The base contains a comma (".jabali,") but no ":2," — must not
		// be mistaken for an info suffix.
		{"comma in base only", "abc.jabali,123", "", false},
	}
	for _, c := range cases {
		flags, has := maildirInfoFlags(c.filename)
		if flags != c.wantFlags || has != c.wantHas {
			t.Errorf("%s: maildirInfoFlags(%q) = (%q,%v), want (%q,%v)",
				c.name, c.filename, flags, has, c.wantFlags, c.wantHas)
		}
	}
}

// TestMaildirFlagsToKeywords — import decode of the flag letters,
// inverse of keywordsToMaildirFlags. Unknown letters ignored.
func TestMaildirFlagsToKeywords(t *testing.T) {
	cases := []struct {
		flags string
		want  map[string]bool
	}{
		{"", map[string]bool{}},
		{"S", map[string]bool{"$seen": true}},
		{"F", map[string]bool{"$flagged": true}},
		{"R", map[string]bool{"$answered": true}},
		{"D", map[string]bool{"$draft": true}},
		{"DFRS", map[string]bool{"$draft": true, "$flagged": true, "$answered": true, "$seen": true}},
		{"FR", map[string]bool{"$flagged": true, "$answered": true}},
		// 'T' (trashed) and 'P' (passed) are not mapped.
		{"ST", map[string]bool{"$seen": true}},
	}
	for _, c := range cases {
		if got := maildirFlagsToKeywords(c.flags); !reflect.DeepEqual(got, c.want) {
			t.Errorf("maildirFlagsToKeywords(%q) = %v, want %v", c.flags, got, c.want)
		}
	}
}

// TestMaildirFlagRoundTrip — the property that matters for fidelity:
// export-encode then import-decode reproduces the original keyword set
// (minus keywords we intentionally don't carry).
func TestMaildirFlagRoundTrip(t *testing.T) {
	inputs := []map[string]bool{
		{"$seen": true},
		{"$flagged": true},
		{"$answered": true},
		{"$draft": true},
		{"$seen": true, "$flagged": true},
		{"$seen": true, "$answered": true, "$draft": true},
		{"$draft": true, "$flagged": true, "$answered": true, "$seen": true},
	}
	for _, in := range inputs {
		flags := keywordsToMaildirFlags(in)
		back := maildirFlagsToKeywords(flags)
		if !reflect.DeepEqual(back, in) {
			t.Errorf("round-trip %v → %q → %v", in, flags, back)
		}
	}
}
