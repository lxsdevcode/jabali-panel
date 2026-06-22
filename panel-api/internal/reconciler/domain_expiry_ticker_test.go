package reconciler

import "testing"

func TestParseWhoisDate(t *testing.T) {
	ok := map[string]string{ // input -> expected YYYY-MM-DD
		"2027-01-23T00:00:00Z":         "2027-01-23",
		"2027-01-23T15:04:05.000Z":     "2027-01-23",
		"2027-01-23":                   "2027-01-23",
		"2027.01.23":                   "2027-01-23",
		"23-Jan-2027":                  "2027-01-23",
		"2027/01/23":                   "2027-01-23",
		"2027-01-23 15:04:05":          "2027-01-23",
		"2027-01-23T00:00:00Z (foo)":   "2027-01-23",
		"  2027-01-23  ":               "2027-01-23",
	}
	for in, want := range ok {
		got := parseWhoisDate(in)
		if got == nil {
			t.Errorf("parseWhoisDate(%q) = nil, want %s", in, want)
			continue
		}
		if got.Format("2006-01-02") != want {
			t.Errorf("parseWhoisDate(%q) = %s, want %s", in, got.Format("2006-01-02"), want)
		}
	}
	for _, bad := range []string{"", "not a date", "n/a", "0000-00-00"} {
		if got := parseWhoisDate(bad); got != nil {
			t.Errorf("parseWhoisDate(%q) = %v, want nil", bad, got)
		}
	}
}
