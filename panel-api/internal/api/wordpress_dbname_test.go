package api

import "testing"

func TestSanitizeDBLabel(t *testing.T) {
	cases := map[string]string{
		"prod.domain.com":   "prod_domain_com",
		"Domain.COM":        "domain_com",
		"a--b..c":           "a_b_c",
		"_lead.trail_":      "lead_trail",
		"my-sub":            "my_sub",
		"x.y.z.example.org": "x_y_z_example_org",
	}
	for in, want := range cases {
		if got := sanitizeDBLabel(in); got != want {
			t.Errorf("sanitizeDBLabel(%q)=%q want %q", in, got, want)
		}
	}
}

func TestFitDBName(t *testing.T) {
	// Short — unchanged.
	if got := fitDBName("terry_wp_", "domain_com", "_db"); got != "terry_wp_domain_com_db" {
		t.Errorf("short: %q", got)
	}
	// Subdir included.
	if got := fitDBName("terry_wp_", "domain_com_sub", "_user"); got != "terry_wp_domain_com_sub_user" {
		t.Errorf("subdir: %q", got)
	}
	// Long FQDN — truncated to fit 64, suffix preserved.
	long := "averylongsubdomain_segment_that_keeps_going_and_going_example_com_extra"
	got := fitDBName("terry_wp_", long, "_db")
	if len(got) > 64 {
		t.Errorf("len=%d > 64: %q", len(got), got)
	}
	if got[:9] != "terry_wp_" || got[len(got)-3:] != "_db" {
		t.Errorf("prefix/suffix lost: %q", got)
	}
}
